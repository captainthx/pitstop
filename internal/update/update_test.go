package update

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"pitstop/internal/model"
)

func TestParseStableVersion(t *testing.T) {
	for _, tc := range []struct {
		input string
		want  [3]int
	}{
		{"v1.2.3", [3]int{1, 2, 3}},
		{"v0.0.0", [3]int{0, 0, 0}},
	} {
		got, err := parseStableVersion(tc.input)
		if err != nil || got != tc.want {
			t.Fatalf("parseStableVersion(%q)=%v,%v want %v,nil", tc.input, got, err, tc.want)
		}
	}
}

func TestParseStableVersionRejectsMalformed(t *testing.T) {
	for _, input := range []string{"", "0.1.0", "v1.2", "v1.2.3-rc1", "v01.2.3", "v+1.2.3", "v1.-0.3"} {
		if _, err := parseStableVersion(input); err == nil {
			t.Fatalf("parseStableVersion(%q) accepted malformed version", input)
		}
	}
}

func TestAcquireLockIsNonblockingAndPersistent(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".pitstop-update.lock")
	first, err := acquireLock(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = syscall.Flock(int(first.Fd()), syscall.LOCK_UN); _ = first.Close() }()
	if _, err := acquireLock(path); err == nil {
		t.Fatal("second updater acquired the lock")
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("lock path was not retained: %v", err)
	}
}

func TestAcquireLockRejectsFIFOWithoutBlocking(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".pitstop-update.lock")
	if err := syscall.Mkfifo(path, 0600); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { _, err := acquireLock(path); done <- err }()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("FIFO lock was accepted")
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("FIFO lock acquisition blocked")
	}
}

func TestChildCancellationKillsProcessGroupPromptly(t *testing.T) {
	path := filepath.Join(t.TempDir(), "child")
	if err := os.WriteFile(path, []byte("#!/bin/sh\n(sleep 10) &\nwait\n"), 0755); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	started := time.Now()
	_, err := child(ctx, path, nil, io.Discard)
	if err == nil || (!strings.Contains(err.Error(), "context canceled") && !strings.Contains(err.Error(), "context deadline exceeded")) {
		t.Fatalf("child error=%v, want cancellation", err)
	}
	if elapsed := time.Since(started); elapsed > 2*time.Second {
		t.Fatalf("child cancellation took %s", elapsed)
	}
}

func TestExecuteSameVersionRefreshUsesCurrentExecutable(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	repo := t.TempDir()
	if out, err := exec.Command("git", "init", "-q", repo).CombinedOutput(); err != nil {
		t.Fatal(string(out))
	}
	current := filepath.Join(t.TempDir(), "pitstop")
	writeFakeCLI(t, current, "0.1.0", false)
	if err := os.MkdirAll(filepath.Join(home, ".agents", "skills", "pitstop-config"), 0700); err != nil {
		t.Fatal(err)
	}
	oldInstaller := runInstaller
	oldLatest := resolveLatestRelease
	defer func() { runInstaller, resolveLatestRelease = oldInstaller, oldLatest }()
	runInstaller = func(context.Context, string, string, io.Writer) error {
		t.Fatal("same-version update downloaded a release")
		return nil
	}
	resolveLatestRelease = func(context.Context) (string, error) { return "v0.1.0", nil }
	result, err := Execute(context.Background(), Options{CurrentVersion: "0.1.0", ExecutablePath: current, Repo: repo, RepoExplicit: true})
	if err != nil {
		t.Fatal(err)
	}
	if result.CLIStatus != "up_to_date" || result.Partial || len(result.Targets) != 1 {
		t.Fatalf("unexpected result: %#v", result)
	}
	for _, target := range result.Targets {
		if target.Status != "updated" {
			t.Errorf("target %s status=%q", target.TargetPath, target.Status)
		}
	}
}

func TestExecutePreflightBlocksBeforeReplacement(t *testing.T) {
	repo := t.TempDir()
	if out, err := exec.Command("git", "init", "-q", repo).CombinedOutput(); err != nil {
		t.Fatal(string(out))
	}
	home := t.TempDir()
	t.Setenv("HOME", home)
	target := filepath.Join(home, ".agents", "skills", "pitstop-config")
	if err := os.MkdirAll(target, 0700); err != nil {
		t.Fatal(err)
	}
	current := filepath.Join(t.TempDir(), "pitstop")
	writeFakeCLI(t, current, "0.1.0", true)
	original, _ := os.ReadFile(current)
	oldInstaller := runInstaller
	oldLatest := resolveLatestRelease
	defer func() { runInstaller, resolveLatestRelease = oldInstaller, oldLatest }()
	runInstaller = func(_ context.Context, _ string, dir string, _ io.Writer) error {
		writeFakeCLI(t, filepath.Join(dir, "pitstop"), "0.2.0", true)
		return nil
	}
	resolveLatestRelease = func(context.Context) (string, error) { return "v0.2.0", nil }
	_, err := Execute(context.Background(), Options{CurrentVersion: "0.1.0", ExecutablePath: current, Repo: repo, RepoExplicit: true})
	if err == nil || !strings.Contains(err.Error(), "preflight") {
		t.Fatalf("error=%v, want preflight block", err)
	}
	got, readErr := os.ReadFile(current)
	if readErr != nil || string(got) != string(original) {
		t.Fatalf("executable changed after blocked preflight: read=%v", readErr)
	}
}

func TestExecutePublishesNewChildToAllExistingSkillScopes(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	repo := t.TempDir()
	if out, err := exec.Command("git", "init", "-q", repo).CombinedOutput(); err != nil {
		t.Fatal(string(out))
	}
	for _, target := range []string{
		filepath.Join(home, ".agents", "skills", "pitstop-config"),
		filepath.Join(home, ".claude", "skills", "pitstop-config"),
		filepath.Join(repo, ".agents", "skills", "pitstop-config"),
		filepath.Join(repo, ".claude", "skills", "pitstop-config"),
	} {
		if err := os.MkdirAll(target, 0700); err != nil {
			t.Fatal(err)
		}
	}
	old := buildFixtureCLI(t, "old", "0.1.0")
	newBinary := buildFixtureCLI(t, "new", "0.2.0")
	oldInstaller := runInstaller
	defer func() { runInstaller = oldInstaller }()
	runInstaller = func(_ context.Context, _ string, dir string, _ io.Writer) error {
		data, err := os.ReadFile(newBinary)
		if err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(dir, "pitstop"), data, 0755)
	}
	result, err := Execute(context.Background(), Options{CurrentVersion: "0.1.0", ExecutablePath: old, Repo: repo, RepoExplicit: true, Version: "v0.2.0", ExplicitVersion: true})
	if err != nil {
		t.Fatal(err)
	}
	if result.CLIStatus != "updated" || len(result.Targets) != 4 {
		t.Fatalf("unexpected result: %#v", result)
	}
	for _, target := range result.Targets {
		marker, readErr := os.ReadFile(filepath.Join(target.TargetPath, "MARKER"))
		if readErr != nil || string(marker) != "new" {
			t.Errorf("target %s marker=%q err=%v", target.TargetPath, marker, readErr)
		}
	}
}

func TestInventorySkipsAbsentSkillDestinations(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	repo := t.TempDir()
	if out, err := exec.Command("git", "init", "-q", repo).CombinedOutput(); err != nil {
		t.Fatal(string(out))
	}
	targets, err := inventory(repo)
	if err != nil {
		t.Fatal(err)
	}
	if len(targets) != 0 {
		t.Fatalf("inventory=%#v, want no absent targets", targets)
	}
}

func TestChildSkillActionsRejectMalformedAndFailedEnvelopes(t *testing.T) {
	targetPath := filepath.Join(t.TempDir(), "skill")
	if err := os.MkdirAll(targetPath, 0700); err != nil {
		t.Fatal(err)
	}
	malformed := filepath.Join(t.TempDir(), "malformed")
	if err := os.WriteFile(malformed, []byte("#!/bin/sh\nprintf '{}'\n"), 0755); err != nil {
		t.Fatal(err)
	}
	target := []updateTarget{{UpdateTarget: model.UpdateTarget{Agent: "codex", Scope: "user", TargetPath: targetPath}}}
	if err := preflightTargets(context.Background(), malformed, target, io.Discard); err == nil {
		t.Fatal("malformed preflight envelope was accepted")
	}
	failed := filepath.Join(t.TempDir(), "failed")
	contents := `#!/bin/sh
printf '%s\n' '{"schemaVersion":1,"command":"skill install","exitCode":0,"result":{"agent":"codex","scope":"user","targetPath":"` + targetPath + `","dryRun":false,"action":"replace","installed":true}}'
exit 2
`
	if err := os.WriteFile(failed, []byte(contents), 0755); err != nil {
		t.Fatal(err)
	}
	targetOne := target[0]
	if err := installTarget(context.Background(), failed, &targetOne, io.Discard); err == nil {
		t.Fatal("failed child with installed=true was accepted")
	}
	if targetOne.Status == "updated" {
		t.Fatal("non-cancelled failed child was reported updated")
	}
}

func TestInstallTargetStartFailureIsClassifiedBeforeMutation(t *testing.T) {
	target := updateTarget{UpdateTarget: model.UpdateTarget{Agent: "codex", Scope: "user", TargetPath: filepath.Join(t.TempDir(), "skill")}}
	err := installTarget(context.Background(), filepath.Join(t.TempDir(), "missing"), &target, io.Discard)
	if err == nil || !isChildStartError(err) || target.Status != "" {
		t.Fatalf("err=%v target=%#v, want unstarted child and untouched target", err, target)
	}
}

func TestExecuteRejectsUnsafeExecutableAndExplicitRepo(t *testing.T) {
	dir := t.TempDir()
	regular := filepath.Join(dir, "pitstop")
	if err := os.WriteFile(regular, []byte("x"), 0755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "link")
	if err := os.Symlink(regular, link); err != nil {
		t.Fatal(err)
	}
	if _, err := validateExecutable(link); err == nil {
		t.Fatal("symlink executable was accepted")
	}
	if _, err := Execute(context.Background(), Options{CurrentVersion: "0.1.0", ExecutablePath: regular, Repo: filepath.Join(dir, "missing"), RepoExplicit: true, Version: "v0.2.0", ExplicitVersion: true}); err == nil {
		t.Fatal("invalid explicit repository was accepted")
	}
}

func TestExecuteCancelledBeforePublicationDoesNotMarkPartial(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	result, err := Execute(ctx, Options{CurrentVersion: "0.1.0", ExecutablePath: os.Args[0], Version: "v0.2.0", ExplicitVersion: true})
	if err == nil || result.Partial {
		t.Fatalf("result=%#v err=%v, want cancellation without partial mutation", result, err)
	}
}

func TestExecuteStageAndVersionFailuresPreserveExecutable(t *testing.T) {
	current := buildFixtureCLI(t, "old", "0.1.0")
	original, err := os.ReadFile(current)
	if err != nil {
		t.Fatal(err)
	}
	oldInstaller := runInstaller
	defer func() { runInstaller = oldInstaller }()
	runInstaller = func(context.Context, string, string, io.Writer) error { return errors.New("fixture download failed") }
	if _, err := Execute(context.Background(), Options{CurrentVersion: "0.1.0", ExecutablePath: current, Version: "v0.2.0", ExplicitVersion: true}); err == nil {
		t.Fatal("staging failure was accepted")
	}
	got, _ := os.ReadFile(current)
	if string(got) != string(original) {
		t.Fatal("staging failure changed executable")
	}
	wrongVersion := buildFixtureCLI(t, "old", "0.1.0")
	runInstaller = func(_ context.Context, _ string, dir string, _ io.Writer) error {
		data, readErr := os.ReadFile(wrongVersion)
		if readErr != nil {
			return readErr
		}
		return os.WriteFile(filepath.Join(dir, "pitstop"), data, 0755)
	}
	if _, err := Execute(context.Background(), Options{CurrentVersion: "0.1.0", ExecutablePath: current, Version: "v0.2.0", ExplicitVersion: true}); err == nil {
		t.Fatal("staged version mismatch was accepted")
	}
	got, _ = os.ReadFile(current)
	if string(got) != string(original) {
		t.Fatal("version failure changed executable")
	}
}

func TestDefaultLatestDoesNotDowngradeNewerCurrent(t *testing.T) {
	current := buildFixtureCLI(t, "old", "0.2.0")
	oldInstaller := runInstaller
	oldLatest := resolveLatestRelease
	defer func() { runInstaller, resolveLatestRelease = oldInstaller, oldLatest }()
	runInstaller = func(context.Context, string, string, io.Writer) error {
		t.Fatal("default latest downgraded current")
		return nil
	}
	resolveLatestRelease = func(context.Context) (string, error) { return "v0.1.0", nil }
	result, err := Execute(context.Background(), Options{CurrentVersion: "0.2.0", ExecutablePath: current})
	if err != nil || result.CLIStatus != "up_to_date" {
		t.Fatalf("result=%#v err=%v", result, err)
	}
}

func TestExecuteRejectsExecutableIdentityChangeDuringStaging(t *testing.T) {
	current := buildFixtureCLI(t, "old", "0.1.0")
	newBinary := buildFixtureCLI(t, "new", "0.2.0")
	oldInstaller := runInstaller
	defer func() { runInstaller = oldInstaller }()
	runInstaller = func(_ context.Context, _ string, dir string, _ io.Writer) error {
		data, err := os.ReadFile(newBinary)
		if err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(dir, "pitstop"), data, 0755); err != nil {
			return err
		}
		return os.WriteFile(current, []byte("changed"), 0755)
	}
	if _, err := Execute(context.Background(), Options{CurrentVersion: "0.1.0", ExecutablePath: current, Version: "v0.2.0", ExplicitVersion: true}); err == nil {
		t.Fatal("executable identity change was accepted")
	}
}

func TestResolveLatestValidatesStableOfficialReleaseURL(t *testing.T) {
	oldTransport := httpClient.Transport
	defer func() { httpClient.Transport = oldTransport }()
	for _, tc := range []struct {
		name, final string
		status      int
		want        string
		ok          bool
	}{
		{name: "stable", final: officialRepo + "/releases/tag/v1.2.3", status: http.StatusOK, want: "v1.2.3", ok: true},
		{name: "prerelease", final: officialRepo + "/releases/tag/v1.2.3-rc1", status: http.StatusOK},
		{name: "wrong-path", final: officialRepo + "/releases/latest", status: http.StatusOK},
		{name: "http-error", final: officialRepo + "/releases/tag/v1.2.3", status: http.StatusBadGateway},
	} {
		t.Run(tc.name, func(t *testing.T) {
			httpClient.Transport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
				finalURL, _ := url.Parse(tc.final)
				return &http.Response{StatusCode: tc.status, Body: io.NopCloser(strings.NewReader("")), Request: &http.Request{URL: finalURL}, Header: make(http.Header)}, nil
			})
			got, err := resolveLatest(context.Background())
			if tc.ok && (err != nil || got != tc.want) {
				t.Fatalf("got=%q err=%v", got, err)
			}
			if !tc.ok && err == nil {
				t.Fatalf("accepted invalid release URL/response: %q", got)
			}
		})
	}
	offHost, _ := url.Parse("https://evil.example/releases/tag/v1.2.3")
	if err := httpClient.CheckRedirect(&http.Request{URL: offHost}, nil); err == nil {
		t.Fatal("off-host redirect accepted")
	}
	insecure, _ := url.Parse("http://github.com/releases/tag/v1.2.3")
	if err := httpClient.CheckRedirect(&http.Request{URL: insecure}, nil); err == nil {
		t.Fatal("insecure redirect accepted")
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }

func buildFixtureCLI(t *testing.T, marker, version string) string {
	t.Helper()
	dir := t.TempDir()
	source := filepath.Join(dir, "main.go")
	program := fmt.Sprintf(`package main
import (
 "encoding/json"; "os"; "path/filepath"
)
func main() {
 args := os.Args[1:]
 if len(args) > 0 && args[0] == "version" { json.NewEncoder(os.Stdout).Encode(map[string]any{"schemaVersion":1,"command":"version","exitCode":0,"result":map[string]any{"version":%q}}); return }
 agent, scope, repo := "codex", "user", ""
 dry := false
 for i := 0; i < len(args); i++ { switch args[i] { case "--agent": agent=args[i+1]; case "--scope": scope=args[i+1]; case "--repo": repo=args[i+1]; case "--dry-run": dry=true } }
 root := os.Getenv("HOME"); if scope == "project" { root = repo }
 target := filepath.Join(root, ".agents", "skills", "pitstop-config"); if agent == "claude" { target = filepath.Join(root, ".claude", "skills", "pitstop-config") }
 if !dry { os.WriteFile(filepath.Join(target,"MARKER"), []byte(%q), 0600) }
 result := map[string]any{"agent":agent,"scope":scope,"targetPath":target,"bundleVersion":"2","files":[]string{},"dryRun":dry,"action":"replace","installed":!dry,"warnings":[]string{}}
 json.NewEncoder(os.Stdout).Encode(map[string]any{"schemaVersion":1,"command":"skill install","exitCode":0,"result":result})
}`, version, marker)
	if err := os.WriteFile(source, []byte(program), 0600); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "pitstop")
	cmd := exec.Command("go", "build", "-o", out, source)
	if data, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build fixture: %v: %s", err, data)
	}
	return out
}

func writeFakeCLI(t *testing.T, path, version string, blocked bool) {
	t.Helper()
	target := filepath.Join(os.Getenv("HOME"), ".agents", "skills", "pitstop-config")
	action := `{"schemaVersion":1,"command":"skill install","exitCode":0,"result":{"agent":"codex","scope":"user","targetPath":"` + target + `","bundleVersion":"2","files":[],"dryRun":false,"action":"replace","installed":true,"warnings":[]}}`
	preflight := `{"schemaVersion":1,"command":"skill install","exitCode":0,"result":{"agent":"codex","scope":"user","targetPath":"` + target + `","bundleVersion":"2","files":[],"dryRun":true,"action":"replace","installed":false,"warnings":[]}}`
	if blocked {
		preflight = `{"schemaVersion":1,"command":"skill install","exitCode":2,"result":{"agent":"codex","scope":"user","targetPath":"` + target + `","dryRun":true,"action":"blocked","installed":false},"error":{"code":"SKILL_INSTALL_FAILED","message":"blocked"}}`
	}
	script := "#!/bin/sh\nif [ \"$1\" = version ]; then printf '%s\\n' '" + `{"schemaVersion":1,"command":"version","exitCode":0,"result":{"version":"` + version + `"}}` + "'; exit 0; fi\nfor arg do if [ \"$arg\" = --dry-run ]; then printf '%s\\n' '" + preflight + "'; exit "
	if blocked {
		script += "2"
	} else {
		script += "0"
	}
	script += "\nfi; done\nprintf '%s\\n' '" + action + "'\ntrue\n"
	if err := os.WriteFile(path, []byte(script), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, 0755); err != nil {
		t.Fatal(err)
	}
}
