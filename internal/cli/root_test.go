package cli

import (
	"bytes"
	"encoding/json"
	"io"
	"os/exec"
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
