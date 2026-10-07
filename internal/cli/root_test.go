package cli

import (
	"bytes"
	"encoding/json"
	"io"
	"os/exec"
	"strings"
	"testing"
)

func TestJSONParseErrorIsOneDocument(t *testing.T) {
	var out, errOut bytes.Buffer
	code := Execute([]string{"run", "--bad", "--json"}, &out, &errOut)
	if code != 2 {
		t.Fatalf("code=%d", code)
	}
	var doc map[string]any
	if err := json.NewDecoder(&out).Decode(&doc); err != nil {
		t.Fatal(err)
	}
	var extra any
	if err := json.NewDecoder(&out).Decode(&extra); err != io.EOF {
		t.Fatalf("extra=%v", err)
	}
}
func TestJSONRunErrorIsOneDocument(t *testing.T) {
	d := t.TempDir()
	if out, err := exec.Command("git", "init", d).CombinedOutput(); err != nil {
		t.Fatal(string(out))
	}
	var out, errOut bytes.Buffer
	code := Execute([]string{"run", "--repo", d, "--json"}, &out, &errOut)
	if code != 2 {
		t.Fatalf("code=%d", code)
	}
	dec := json.NewDecoder(&out)
	var doc map[string]any
	if err := dec.Decode(&doc); err != nil {
		t.Fatal(err)
	}
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		t.Fatalf("extra=%v", err)
	}
}

func TestJSONEqualsFlagParseErrorIsOneDocument(t *testing.T) {
	var out, errOut bytes.Buffer
	if Execute([]string{"--repo", "/missing", "run", "--bad", "--json=true"}, &out, &errOut) != 2 {
		t.Fatal("expected exit2")
	}
	var doc map[string]any
	dec := json.NewDecoder(&out)
	if err := dec.Decode(&doc); err != nil {
		t.Fatal(err)
	}
	if doc["command"] != "run" {
		t.Fatal(doc)
	}
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		t.Fatal(err)
	}
}
func TestDoctorBlockedReturnsPreflightExit(t *testing.T) {
	var out, errOut bytes.Buffer
	if code := Execute([]string{"doctor", "--repo", t.TempDir(), "--json"}, &out, &errOut); code != 2 {
		t.Fatalf("blocked doctor exit=%d", code)
	}
	var doc map[string]any
	if err := json.Unmarshal(out.Bytes(), &doc); err != nil {
		t.Fatal(err)
	}
	if doc["exitCode"] != float64(2) {
		t.Fatal(doc)
	}
}

func TestHelp(t *testing.T) {
	t.Setenv("PATH", "") // Help must not launch Git, act, or Docker.
	missingRepo := t.TempDir() + "/missing"
	tests := []struct {
		name string
		args []string
		want []string
	}{
		{"root", []string{"-h"}, []string{
			"Run GitHub Actions checks locally before sending changes to CI.",
			"uncommitted changes", "reports and logs", "Ctrl+C",
			"Check repository, configuration, workflow, act, and Docker readiness",
			"List workflow jobs, selection, and eligibility",
			"Run selected workflow jobs in an isolated working-tree copy",
			"Show version and build information", "--repo", "pitstop run --repo /path/to/repo",
		}},
		{"doctor", []string{"doctor", "-h"}, []string{
			"act 0.2.89", ".pitstop.json", "pitstop doctor --repo /path/to/repo", "--json", "--repo",
		}},
		{"list", []string{"list", "-h"}, []string{
			"blocked", ".github/workflows/ci.yml", "pitstop list --repo /path/to/repo", "--json", "--repo",
		}},
		{"run", []string{"run", "-h"}, []string{
			"already selected in .pitstop.json", "Ctrl+C", "reports and logs",
			"pitstop run --repo /path/to/repo --job lint --job test",
			"--job", "--timeout", "--arch", "--force-integration", "--json", "--repo", "20m0s",
		}},
		{"version", []string{"version", "-h"}, []string{
			"Show version and build information", "pitstop version --json", "--json",
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var shortOut, shortErr, longOut, longErr bytes.Buffer
			if code := Execute(tt.args, &shortOut, &shortErr); code != 0 {
				t.Fatalf("exit=%d stderr=%s", code, shortErr.String())
			}
			longArgs := append([]string(nil), tt.args...)
			longArgs[len(longArgs)-1] = "--help"
			if code := Execute(longArgs, &longOut, &longErr); code != 0 {
				t.Fatalf("long help exit=%d", code)
			}
			if shortErr.Len() != 0 || longErr.Len() != 0 || shortOut.String() != longOut.String() {
				t.Fatal("help aliases must produce identical stdout and empty stderr")
			}
			for _, want := range tt.want {
				if !strings.Contains(shortOut.String(), want) {
					t.Errorf("help missing %q", want)
				}
			}
		})
	}
	for _, args := range [][]string{
		{},
		{"help", "run"},
		{"doctor", "--repo", missingRepo, "-h"},
		{"run", "--repo", missingRepo, "--json", "-h"},
	} {
		var out, errOut bytes.Buffer
		if code := Execute(args, &out, &errOut); code != 0 || errOut.Len() != 0 {
			t.Fatalf("args=%v exit=%d stderr=%s", args, code, errOut.String())
		}
		want := "Run GitHub Actions checks locally before sending changes to CI."
		if len(args) > 0 {
			want = "pitstop " + args[0] + " --repo /path/to/repo"
			if args[0] == "help" {
				want = "pitstop run --repo /path/to/repo"
			}
		}
		if !strings.Contains(out.String(), want) || !strings.Contains(out.String(), "Usage:") {
			t.Fatalf("args=%v missing help: %s", args, out.String())
		}
	}
}
