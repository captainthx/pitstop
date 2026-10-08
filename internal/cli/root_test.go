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
	"strings"
	"testing"
)

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, errors.New("stdout failed") }

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
}

func TestValidateJSONFailures(t *testing.T) {
	d := t.TempDir()
	if out, err := exec.Command("git", "init", "-q", d).CombinedOutput(); err != nil {
		t.Fatal(string(out))
	}
	for _, input := range []string{
		"not-json",
		`{"schemaVersion":2,"runner":{"image":"alpine:3.22","architecture":"linux/arm64"},"jobs":["missing"],"checks":[{"id":"verify","steps":[{"id":"check","run":"true"}]}]}`,
	} {
		if err := os.WriteFile(filepath.Join(d, ".pitstop.json"), []byte(input), 0600); err != nil {
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
