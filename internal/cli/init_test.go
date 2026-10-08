package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"pitstop/internal/config"
	"pitstop/internal/model"
)

func cliInitRepo(t *testing.T) string {
	t.Helper()
	d := t.TempDir()
	if out, err := exec.Command("git", "init", "-q", d).CombinedOutput(); err != nil {
		t.Fatal(string(out))
	}
	return d
}

func decodeInitEnvelope(t *testing.T, data []byte) model.Envelope {
	t.Helper()
	dec := json.NewDecoder(bytes.NewReader(data))
	var envelope model.Envelope
	if err := dec.Decode(&envelope); err != nil {
		t.Fatal(err)
	}
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		t.Fatalf("expected one JSON envelope, got %v", err)
	}
	return envelope
}

func TestInitCLIJSONNoHEADAndPermissions(t *testing.T) {
	d := cliInitRepo(t)
	var out, errOut bytes.Buffer
	if code := Execute([]string{"init", "--repo", d, "--preset", "custom", "--image", "alpine:3.22", "--command", "echo a,b\necho $HOME", "--arch", "linux/amd64", "--json"}, &out, &errOut); code != 0 {
		t.Fatalf("code=%d stdout=%s stderr=%s", code, out.String(), errOut.String())
	}
	envelope := decodeInitEnvelope(t, out.Bytes())
	if envelope.Command != "init" || envelope.ExitCode != 0 {
		t.Fatalf("envelope=%#v", envelope)
	}
	path := filepath.Join(d, ".pitstop.json")
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0600 {
		t.Fatalf("mode=%o", info.Mode().Perm())
	}
	cfg, err := config.Load(d)
	if err != nil || cfg.Checks[0].Steps[0].Run != "echo a,b\necho $HOME" {
		t.Fatalf("config=%#v err=%v", cfg, err)
	}
}

func TestInitCLIDryRunExistingAndBrokenSymlink(t *testing.T) {
	t.Run("existing", func(t *testing.T) {
		d := cliInitRepo(t)
		path := filepath.Join(d, ".pitstop.json")
		if err := os.WriteFile(path, []byte("sentinel"), 0600); err != nil {
			t.Fatal(err)
		}
		var out, errOut bytes.Buffer
		if code := Execute([]string{"init", "--repo", d, "--dry-run", "--json"}, &out, &errOut); code != 0 {
			t.Fatalf("code=%d stdout=%s stderr=%s", code, out.String(), errOut.String())
		}
		e := decodeInitEnvelope(t, out.Bytes())
		result := e.Result.(map[string]any)
		if result["destinationExists"] != true || result["written"] != false {
			t.Fatalf("result=%#v", result)
		}
		got, _ := os.ReadFile(path)
		if string(got) != "sentinel" {
			t.Fatalf("destination changed: %q", got)
		}
	})
	t.Run("broken symlink", func(t *testing.T) {
		d := cliInitRepo(t)
		path := filepath.Join(d, ".pitstop.json")
		if err := os.Symlink(filepath.Join(d, "missing"), path); err != nil {
			t.Fatal(err)
		}
		var out, errOut bytes.Buffer
		if code := Execute([]string{"init", "--repo", d, "--dry-run", "--json"}, &out, &errOut); code != 0 {
			t.Fatalf("code=%d stdout=%s stderr=%s", code, out.String(), errOut.String())
		}
		result := decodeInitEnvelope(t, out.Bytes()).Result.(map[string]any)
		if result["destinationExists"] != true {
			t.Fatalf("result=%#v", result)
		}
	})
}

func TestInitCLIInputErrorsAndHelp(t *testing.T) {
	d := cliInitRepo(t)
	for _, args := range [][]string{
		{"init", "--repo", d, "unexpected", "--json"},
		{"init", "--repo", d, "--unknown", "--json"},
		{"init", "--repo", d, "--preset"},
		{"init", "--repo", d, "--json", "--preset"},
		{"init", "--repo", d, "--preset", "nope", "--json"},
		{"init", "--repo", d, "--arch", "darwin/arm64", "--json"},
		{"init", "--repo", d, "--preset", "custom", "--image", "alpine:3.22", "--json"},
	} {
		var out, errOut bytes.Buffer
		if code := Execute(args, &out, &errOut); code != 2 {
			t.Fatalf("args=%v code=%d stdout=%q stderr=%q", args, code, out.String(), errOut.String())
		}
		if strings.Contains(strings.Join(args, " "), "--json") {
			e := decodeInitEnvelope(t, out.Bytes())
			if e.Command != "init" || e.ExitCode != 2 || e.Error == nil {
				t.Fatalf("args=%v envelope=%#v", args, e)
			}
		}
	}
	var out, errOut bytes.Buffer
	if code := Execute([]string{"init", "--help"}, &out, &errOut); code != 0 {
		t.Fatalf("help code=%d stderr=%s", code, errOut.String())
	}
	for _, preset := range []string{"base", "node-npm", "node-pnpm", "node-yarn-classic", "node-yarn-berry", "go", "python-pip", "python-uv", "java-maven", "java-gradle", "dotnet", "rust", "php-composer", "ruby-bundler", "custom"} {
		if !strings.Contains(out.String(), preset) {
			t.Fatalf("help omitted preset %q: %s", preset, out.String())
		}
	}
}

func TestInitCLICancelledBeforeWrite(t *testing.T) {
	d := cliInitRepo(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	var out, errOut bytes.Buffer
	if code := ExecuteContext(ctx, []string{"init", "--repo", d, "--json"}, &out, &errOut); code != 130 {
		t.Fatalf("code=%d stdout=%s stderr=%s", code, out.String(), errOut.String())
	}
	e := decodeInitEnvelope(t, out.Bytes())
	if e.Error == nil || e.Error.Code != "CANCELLED" || e.ExitCode != 130 {
		t.Fatalf("envelope=%#v", e)
	}
	if _, err := os.Stat(filepath.Join(d, ".pitstop.json")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("destination exists: %v", err)
	}
}

func TestInitCLICancellationDuringGitResolution(t *testing.T) {
	d := t.TempDir()
	marker := filepath.Join(d, "started")
	fake := t.TempDir()
	gitScript := filepath.Join(fake, "git")
	script := "#!/bin/sh\nprintf started > '" + marker + "'\nsleep 1\nexit 1\n"
	if err := os.WriteFile(gitScript, []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", fake+string(os.PathListSeparator)+os.Getenv("PATH"))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		deadline := time.Now().Add(time.Second)
		for time.Now().Before(deadline) {
			if _, err := os.Stat(marker); err == nil {
				cancel()
				return
			}
			time.Sleep(time.Millisecond)
		}
	}()
	var out, errOut bytes.Buffer
	if code := ExecuteContext(ctx, []string{"init", "--repo", t.TempDir(), "--json"}, &out, &errOut); code != 130 {
		t.Fatalf("code=%d stdout=%s stderr=%s", code, out.String(), errOut.String())
	}
	e := decodeInitEnvelope(t, out.Bytes())
	if e.Error == nil || e.Error.Code != "CANCELLED" || e.ExitCode != 130 {
		t.Fatalf("envelope=%#v", e)
	}
}

func TestInitCLIStdoutFailure(t *testing.T) {
	d := cliInitRepo(t)
	var errOut bytes.Buffer
	if code := Execute([]string{"init", "--repo", d, "--dry-run", "--json"}, failingWriter{}, &errOut); code != 2 || !strings.Contains(errOut.String(), "stdout failed") {
		t.Fatalf("code=%d stderr=%q", code, errOut.String())
	}
}
