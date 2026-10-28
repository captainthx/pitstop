package cli

import (
	"bytes"
	"encoding/json"
	"testing"

	"pitstop/internal/model"
)

func TestUpdateRejectsMalformedVersionBeforeDownload(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Chdir(t.TempDir())
	var out, errOut bytes.Buffer
	code := Execute([]string{"update", "--version", "v1.2", "--json"}, &out, &errOut)
	if code != 2 {
		t.Fatalf("code=%d stdout=%q stderr=%q", code, out.String(), errOut.String())
	}
	var env model.Envelope
	if err := json.Unmarshal(out.Bytes(), &env); err != nil {
		t.Fatalf("decode: %v; output=%q", err, out.String())
	}
	if env.Command != "update" || env.ExitCode != 2 || env.Error == nil || env.Error.Code != "UPDATE_INVALID_VERSION" {
		t.Fatalf("envelope=%#v", env)
	}
}

func TestUpdateRejectsEmptyExplicitVersion(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Chdir(t.TempDir())
	var out, errOut bytes.Buffer
	code := Execute([]string{"update", "--version=", "--json"}, &out, &errOut)
	if code != 2 {
		t.Fatalf("code=%d stdout=%q stderr=%q", code, errOut.String(), out.String())
	}
	var env model.Envelope
	if err := json.Unmarshal(out.Bytes(), &env); err != nil || env.Error == nil || env.Error.Code != "UPDATE_INVALID_VERSION" {
		t.Fatalf("envelope=%#v err=%v", env, err)
	}
}
