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
	"pitstop/internal/model"
	"reflect"
	"strings"
	"testing"
)

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, errors.New("stdout failed") }

type laterFailWriter struct {
	writes int
}

func (w *laterFailWriter) Write(p []byte) (int, error) {
	w.writes++
	if w.writes > 1 {
		return 0, errors.New("later stdout failed")
	}
	return len(p), nil
}

type cancelAfterErrors struct {
	context.Context
	calls int
}

func (c *cancelAfterErrors) Err() error {
	c.calls++
	if c.calls >= 3 {
		return context.Canceled
	}
	return nil
}

func treeState(root string) (map[string]string, error) {
	state := map[string]string{}
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if os.IsNotExist(err) {
			return filepath.SkipDir
		}
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		value := info.Mode().String()
		if info.Mode().IsRegular() {
			b, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			value += ":" + string(b)
		}
		state[rel] = value
		return nil
	})
	return state, err
}

func TestValidateOfflineAndNoWrites(t *testing.T) {
	d := t.TempDir()
	if out, err := exec.Command("git", "init", "-q", d).CombinedOutput(); err != nil {
		t.Fatal(string(out))
	}
	marker := filepath.Join(t.TempDir(), "marker")
	config := `{"schemaVersion":2,"runner":{"image":"alpine:3.22","architecture":"linux/arm64"},"jobs":["verify"],"checks":[{"id":"verify","steps":[{"id":"check","run":"printf marker > ` + marker + `"},{"id":"second","run":"true"}]}]}`
	if err := os.WriteFile(filepath.Join(d, ".pitstop.json"), []byte(config), 0600); err != nil {
		t.Fatal(err)
	}
	fakeBin := t.TempDir()
	for _, name := range []string{"docker", "node", "go"} {
		if err := os.WriteFile(filepath.Join(fakeBin, name), []byte("#!/bin/sh\nprintf called > '"+marker+"'\n"), 0700); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", fakeBin+string(os.PathListSeparator)+os.Getenv("PATH"))
	repoBefore, err := treeState(d)
	if err != nil {
		t.Fatal(err)
	}
	cacheRoot, err := os.UserCacheDir()
	if err != nil {
		t.Fatal(err)
	}
	cacheBefore, err := treeState(filepath.Join(cacheRoot, "pitstop"))
	if err != nil {
		t.Fatal(err)
	}
	var out, errOut bytes.Buffer
	if code := ExecuteContext(context.Background(), []string{"validate", "--repo", d, "--json"}, &out, &errOut); code != 0 {
		t.Fatal(code, out.String(), errOut.String())
	}
	var envelope model.Envelope
	if err := json.Unmarshal(out.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	result, ok := envelope.Result.(map[string]any)
	if !ok || result["valid"] != true || result["checkCount"] != float64(1) || result["stepCount"] != float64(2) {
		t.Fatalf("unexpected result: %#v", envelope.Result)
	}
	if _, err := os.Stat(marker); err == nil {
		t.Fatal("validate executed a configured command")
	}
	repoAfter, err := treeState(d)
	if err != nil {
		t.Fatal(err)
	}
	cacheAfter, err := treeState(filepath.Join(cacheRoot, "pitstop"))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(repoBefore, repoAfter) || !reflect.DeepEqual(cacheBefore, cacheAfter) {
		t.Fatalf("validate changed repo/cache state: repo=%v cache=%v", repoAfter, cacheAfter)
	}
}

func TestValidateJSONFailures(t *testing.T) {
	d := t.TempDir()
	if out, err := exec.Command("git", "init", "-q", d).CombinedOutput(); err != nil {
		t.Fatal(string(out))
	}
	for _, input := range [][]byte{
		[]byte("not-json"),
		[]byte(`{"schemaVersion":2,"runner":{"image":"alpine:3.22","architecture":"linux/arm64"},"jobs":["missing"],"checks":[{"id":"verify","steps":[{"id":"check","run":"true"}]}]}`),
		[]byte(`{"schemaVersion":2,"runner":{"image":"alpine:3.22","architecture":"linux/arm64"},"jobs":["verify"],"checks":[{"id":"verify","steps":[{"id":"check","run":"true","id":"other"}]}]}`),
		[]byte(strings.Repeat("x", model.MaxConfigBytes+1)),
	} {
		if err := os.WriteFile(filepath.Join(d, ".pitstop.json"), input, 0600); err != nil {
			t.Fatal(err)
		}
		var out, errOut bytes.Buffer
		if code := Execute([]string{"validate", "--repo", d, "--json"}, &out, &errOut); code != 2 {
			t.Fatalf("code=%d stdout=%s stderr=%s", code, out.String(), errOut.String())
		}
		var envelope model.Envelope
		decoder := json.NewDecoder(&out)
		if err := decoder.Decode(&envelope); err != nil {
			t.Fatal(err)
		}
		var extra any
		if err := decoder.Decode(&extra); err != io.EOF {
			t.Fatalf("expected one envelope, got %v", err)
		}
		if envelope.Command != "validate" || envelope.Error == nil || envelope.Error.Code != "CONFIG_INVALID" {
			t.Fatalf("unexpected envelope: %#v", envelope)
		}
		result, ok := envelope.Result.(map[string]any)
		if !ok || result["valid"] != false {
			t.Fatalf("unexpected result: %#v", envelope.Result)
		}
		issues, ok := result["issues"].([]any)
		if !ok || len(issues) != 1 {
			t.Fatalf("unexpected issues: %#v", result["issues"])
		}
	}
	if err := os.Remove(filepath.Join(d, ".pitstop.json")); err != nil {
		t.Fatal(err)
	}
	var out, errOut bytes.Buffer
	if code := Execute([]string{"validate", "--repo", d, "--json"}, &out, &errOut); code != 2 {
		t.Fatalf("missing config code=%d stdout=%s stderr=%s", code, out.String(), errOut.String())
	}
	var envelope model.Envelope
	decoder := json.NewDecoder(&out)
	if err := decoder.Decode(&envelope); err != nil {
		t.Fatal(err)
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF || envelope.Result == nil {
		t.Fatalf("missing config envelope=%#v trailing=%v", envelope, err)
	}
	missingResult, ok := envelope.Result.(map[string]any)
	if !ok || missingResult["valid"] != false || len(missingResult["issues"].([]any)) != 1 {
		t.Fatalf("missing config result=%#v", envelope.Result)
	}
}

func TestValidateCancellationAndFlagValues(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	var out, errOut bytes.Buffer
	if code := ExecuteContext(ctx, []string{"validate", "--repo", "run", "--json"}, &out, &errOut); code != 130 {
		t.Fatalf("cancelled code=%d stdout=%s stderr=%s", code, out.String(), errOut.String())
	}
	var envelope model.Envelope
	if err := json.Unmarshal(out.Bytes(), &envelope); err != nil || envelope.Command != "validate" || envelope.ExitCode != 130 {
		t.Fatalf("cancelled envelope=%#v err=%v", envelope, err)
	}
	var syntaxOut, syntaxErr bytes.Buffer
	if code := Execute([]string{"validate", "--repo=--json"}, &syntaxOut, &syntaxErr); code != 2 || syntaxOut.Len() != 0 {
		t.Fatalf("literal flag value emitted unexpected output: code=%d stdout=%q stderr=%q", code, syntaxOut.String(), syntaxErr.String())
	}
}

func TestValidateOutputFailureDoesNotWriteSecondEnvelope(t *testing.T) {
	d := t.TempDir()
	if out, err := exec.Command("git", "init", "-q", d).CombinedOutput(); err != nil {
		t.Fatal(string(out))
	}
	config := `{"schemaVersion":2,"runner":{"image":"alpine:3.22","architecture":"linux/arm64"},"jobs":["verify"],"checks":[{"id":"verify","steps":[{"id":"check","run":"true"}]}]}`
	if err := os.WriteFile(filepath.Join(d, ".pitstop.json"), []byte(config), 0600); err != nil {
		t.Fatal(err)
	}
	var errOut bytes.Buffer
	if code := Execute([]string{"validate", "--repo", d, "--json"}, failingWriter{}, &errOut); code != 2 || !strings.Contains(errOut.String(), "stdout failed") {
		t.Fatalf("code=%d stderr=%q", code, errOut.String())
	}
}

func TestValidateCancellationWinsOverFailedLoad(t *testing.T) {
	d := t.TempDir()
	if out, err := exec.Command("git", "init", "-q", d).CombinedOutput(); err != nil {
		t.Fatal(string(out))
	}
	if err := os.WriteFile(filepath.Join(d, ".pitstop.json"), []byte("not-json"), 0600); err != nil {
		t.Fatal(err)
	}
	ctx := &cancelAfterErrors{Context: context.Background()}
	var out, errOut bytes.Buffer
	if code := ExecuteContext(ctx, []string{"validate", "--repo", d, "--json"}, &out, &errOut); code != 130 {
		t.Fatalf("code=%d stdout=%s stderr=%s", code, out.String(), errOut.String())
	}
	var envelope model.Envelope
	if err := json.Unmarshal(out.Bytes(), &envelope); err != nil || envelope.ExitCode != 130 || envelope.Error == nil || envelope.Error.Code != "CANCELLED" {
		t.Fatalf("envelope=%#v err=%v", envelope, err)
	}
}

func TestExistingHumanCommandPropagatesLaterWriteFailure(t *testing.T) {
	d := t.TempDir()
	if out, err := exec.Command("git", "init", "-q", d).CombinedOutput(); err != nil {
		t.Fatal(string(out))
	}
	config := `{"schemaVersion":2,"runner":{"image":"alpine:3.22","architecture":"linux/arm64"},"jobs":["verify","lint"],"checks":[{"id":"verify","steps":[{"id":"check","run":"true"}]},{"id":"lint","steps":[{"id":"check","run":"true"}]}]}`
	if err := os.WriteFile(filepath.Join(d, ".pitstop.json"), []byte(config), 0600); err != nil {
		t.Fatal(err)
	}
	var errOut bytes.Buffer
	writer := &laterFailWriter{}
	if code := Execute([]string{"list", "--repo", d}, writer, &errOut); code != 2 || !strings.Contains(errOut.String(), "later stdout failed") {
		t.Fatalf("code=%d writes=%d stderr=%q", code, writer.writes, errOut.String())
	}
}

func TestValidateCommandFlagsAndEnvelope(t *testing.T) {
	d := t.TempDir()
	if out, err := exec.Command("git", "init", "-q", d).CombinedOutput(); err != nil {
		t.Fatal(string(out))
	}
	config := `{"schemaVersion":2,"runner":{"image":"alpine:3.22","architecture":"linux/arm64"},"jobs":["verify"],"checks":[{"id":"verify","steps":[{"id":"check","run":"true"}]}]}`
	if err := os.WriteFile(filepath.Join(d, ".pitstop.json"), []byte(config), 0600); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		args    []string
		code    int
		json    bool
		command string
	}{
		{[]string{"validate", "--json=true", "--repo", d}, 0, true, "validate"},
		{[]string{"--repo", d, "validate", "--job", "verify", "--arch", "linux/amd64", "--json"}, 0, true, "validate"},
		{[]string{"validate", "--repo", d, "--json=false"}, 0, false, ""},
		{[]string{"validate", "--repo", d, "--", "--json"}, 2, false, ""},
		{[]string{"validate", "--repo", d, "unexpected", "--json"}, 2, true, "validate"},
		{[]string{"validate", "--repo", d, "--unknown", "--json"}, 2, true, "validate"},
		{[]string{"validate", "--repo", "--json"}, 2, false, "validate"},
	} {
		var out, errOut bytes.Buffer
		if code := Execute(tc.args, &out, &errOut); code != tc.code {
			t.Fatalf("args=%v code=%d want=%d stdout=%q stderr=%q", tc.args, code, tc.code, out.String(), errOut.String())
		}
		if tc.json {
			var envelope model.Envelope
			if err := json.Unmarshal(out.Bytes(), &envelope); err != nil || envelope.Command != tc.command {
				t.Fatalf("args=%v envelope=%#v err=%v", tc.args, envelope, err)
			}
		} else if out.Len() > 0 && strings.Contains(out.String(), "\"schemaVersion\"") {
			t.Fatalf("args=%v unexpectedly emitted JSON: %q", tc.args, out.String())
		}
	}
	for _, name := range []string{"doctor", "list", "run", "version"} {
		var out, errOut bytes.Buffer
		if code := Execute([]string{name, "--unknown", "--json"}, &out, &errOut); code != 2 {
			t.Fatalf("%s code=%d stdout=%q stderr=%q", name, code, out.String(), errOut.String())
		}
		var envelope model.Envelope
		if err := json.Unmarshal(out.Bytes(), &envelope); err != nil || envelope.Command != name {
			t.Fatalf("%s envelope=%#v err=%v", name, envelope, err)
		}
	}
}

func TestHelpDescribesDockerChecks(t *testing.T) {
	var out, errOut bytes.Buffer
	if code := Execute([]string{"-h"}, &out, &errOut); code != 0 {
		t.Fatal(code)
	}
	for _, want := range []string{"Docker", "fresh", "schemaVersion 2"} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("missing %q: %s", want, out.String())
		}
	}
}
func TestRunRejectsRemovedActFlag(t *testing.T) {
	var out, errOut bytes.Buffer
	if code := Execute([]string{"run", "--force-integration"}, &out, &errOut); code != 2 || (!strings.Contains(errOut.String(), "unknown flag") && !strings.Contains(out.String(), "unknown flag")) {
		t.Fatalf("unexpected: %d stdout=%s stderr=%s", code, out.String(), errOut.String())
	}
}
func TestVersionJSONIsOneEnvelope(t *testing.T) {
	var out, errOut bytes.Buffer
	if code := Execute([]string{"version", "--json"}, &out, &errOut); code != 0 {
		t.Fatal(code, errOut.String())
	}
	var e map[string]any
	if err := json.Unmarshal(out.Bytes(), &e); err != nil {
		t.Fatal(err)
	}
	if e["schemaVersion"] != float64(1) || e["command"] != "version" {
		t.Fatal(e)
	}
}
func TestListReadsChecksConfig(t *testing.T) {
	d := t.TempDir()
	if out, err := exec.Command("git", "init", "-q", d).CombinedOutput(); err != nil {
		t.Fatal(string(out))
	}
	b := []byte(`{"schemaVersion":2,"runner":{"image":"alpine:3.22","architecture":"linux/arm64"},"jobs":["verify"],"checks":[{"id":"verify","steps":[{"id":"check","run":"true"}]}]}`)
	if err := os.WriteFile(filepath.Join(d, ".pitstop.json"), b, 0600); err != nil {
		t.Fatal(err)
	}
	var out, errOut bytes.Buffer
	if code := Execute([]string{"list", "--repo", d, "--json"}, &out, &errOut); code != 0 {
		t.Fatal(code, errOut.String())
	}
	if !strings.Contains(out.String(), `"configPath":".pitstop.json"`) {
		t.Fatal(out.String())
	}
}
