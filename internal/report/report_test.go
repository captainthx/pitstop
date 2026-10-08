package report

import (
	"bytes"
	"encoding/json"
	"io"
	"pitstop/internal/model"
	"strings"
	"testing"
)

func TestExitPrecedence(t *testing.T) {
	for _, tc := range []struct {
		s    []string
		c    bool
		want int
	}{{[]string{"passed"}, false, 0}, {[]string{"failed", "blocked"}, false, 1}, {[]string{"passed", "blocked"}, false, 2}, {[]string{"skipped"}, false, 2}, {nil, false, 2}, {[]string{"failed"}, true, 130}} {
		var jobs []model.JobResult
		for _, s := range tc.s {
			jobs = append(jobs, model.JobResult{Status: s})
		}
		if got := ExitCode(jobs, tc.c); got != tc.want {
			t.Fatalf("got %d want %d", got, tc.want)
		}
	}
}

func TestHumanSkillInstallErrorShowsRecoveryWarning(t *testing.T) {
	var b bytes.Buffer
	e := model.Envelope{
		SchemaVersion: model.SchemaVersion,
		Command:       "skill install",
		ExitCode:      2,
		Result: model.SkillInstallResult{
			Agent: "codex", Scope: "project", Action: "replace",
			Warnings: []string{"retained backup for recovery: /tmp/backup"},
		},
		Error: &model.ErrorInfo{Code: "SKILL_INSTALL_FAILED", Message: "publish failed and rollback failed"},
	}
	if err := Human(&b, e); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(b.String(), "/tmp/backup") {
		t.Fatalf("human output omitted recovery path: %q", b.String())
	}
}
func TestEnvelopeIsOneDocument(t *testing.T) {
	var b bytes.Buffer
	want := model.Envelope{SchemaVersion: 1, Command: "run", ExitCode: 2, Error: &model.ErrorInfo{Code: "CONFIG_INVALID", Message: "missing config"}}
	if err := WriteEnvelope(&b, want); err != nil {
		t.Fatal(err)
	}
	var got model.Envelope
	if err := json.NewDecoder(&b).Decode(&got); err != nil {
		t.Fatal(err)
	}
	if got.Error == nil || got.Error.Code != "CONFIG_INVALID" {
		t.Fatalf("%#v", got)
	}
	var extra any
	if err := json.NewDecoder(&b).Decode(&extra); err != io.EOF {
		t.Fatalf("trailing document: %v", err)
	}
}
