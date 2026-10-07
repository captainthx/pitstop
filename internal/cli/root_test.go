package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

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
