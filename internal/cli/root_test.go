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
