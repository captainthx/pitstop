package report

import (
	"bytes"
	"encoding/json"
	"io"
	"pitstop/internal/model"
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
