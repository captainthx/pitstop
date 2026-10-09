package config

import (
	"encoding/json"
	"errors"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"

	"pitstop/internal/model"
)

func TestParseRejectsAmbiguousInput(t *testing.T) {
	cases := []string{
		`null`, `[]`, `{} {}`, validConfig + ` garbage`,
		strings.Replace(validConfig, `"schemaVersion":2`, `"schemaVersion":2,"schemaVersion":1`, 1),
		strings.Replace(validConfig, `"schemaVersion":2`, `"SchemaVersion":2`, 1),
		strings.Replace(validConfig, `"id":"check"`, `"id":"check","\u0069d":"other"`, 1),
		strings.Replace(validConfig, `"run":"true"`, `"name":null,"run":"true"`, 1),
		strings.Replace(validConfig, `"run":"true"`, `"run":"\ud800"`, 1),
		strings.Replace(validConfig, `"schemaVersion":2`, `"schemaVersion":2e0`, 1),
	}
	for _, input := range cases {
		got, err := Parse([]byte(input))
		if err == nil {
			t.Fatalf("accepted ambiguous input %q as %#v", input, got)
		}
		if !reflect.DeepEqual(got, model.Config{}) {
			t.Fatalf("returned partial config for %q: %#v", input, got)
		}
	}
}

func TestParseStrictShape(t *testing.T) {
	cases := []string{
		strings.Replace(validConfig, `"runner":`, `"unknown":1,"runner":`, 1),
		strings.Replace(validConfig, `"runner":`, `"Runner":`, 1),
		strings.Replace(validConfig, `"runner":{"image"`, `"runner":{"image":null,"image"`, 1),
		strings.Replace(validConfig, `"runner":{"image":"alpine:3.22"`, `"runner":{"image":1`, 1),
		strings.Replace(validConfig, `"runner":{"image":"alpine:3.22","architecture":"linux/arm64"}`, `"runner":{}`, 1),
		strings.Replace(validConfig, `"checks":[{`, `"checks":[{"unknown":1,`, 1),
		strings.Replace(validConfig, `"checks":[{"id"`, `"checks":[{"ID"`, 1),
		strings.Replace(validConfig, `"checks":[{"id":"verify"`, `"checks":[{"id":null`, 1),
		strings.Replace(validConfig, `"checks":[{"id":"verify"`, `"checks":[{"id":1`, 1),
		strings.Replace(validConfig, `"checks":[{"id":"verify","steps"`, `"checks":[{"id":"verify"}`, 1),
		strings.Replace(validConfig, `"steps"`, `"ſteps"`, 1),
		strings.Replace(validConfig, `"steps":[{"id"`, `"steps":[{"ID"`, 1),
		strings.Replace(validConfig, `"steps":[{"id":"check"`, `"steps":[{"id":null`, 1),
		strings.Replace(validConfig, `"steps":[{"id":"check"`, `"steps":[{"id":1`, 1),
		strings.Replace(validConfig, `"run":"true"`, `"run":1`, 1),
		strings.Replace(validConfig, `"schemaVersion":2`, `"schemaVersion":"2"`, 1),
		strings.Replace(validConfig, `"schemaVersion":2`, `"schemaVersion":2.0`, 1),
		strings.Replace(validConfig, `"schemaVersion":2`, `"schemaVersion":999999999999999999999999`, 1),
		`1`, "\ufeff" + validConfig, strings.Replace(validConfig, `"run":"true"`, `"run":"true",`, 1),
		strings.Replace(validConfig, `{"schemaVersion"`, `{"schemaVersion"`, 1) + ` // comment`,
	}
	for _, input := range cases {
		if got, err := Parse([]byte(input)); err == nil || !reflect.DeepEqual(got, model.Config{}) {
			t.Fatalf("accepted strict-shape input %q as %#v (err=%v)", input, got, err)
		}
	}
}

func TestParseUnicode(t *testing.T) {
	valid := []string{
		strings.Replace(validConfig, `"run":"true"`, `"run":"สวัสดี"`, 1),
		strings.Replace(validConfig, `"run":"true"`, `"run":"\ud83d\ude00"`, 1),
		strings.Replace(validConfig, `"run":"true"`, "\"run\":\"�\"", 1),
	}
	for _, input := range valid {
		if got, err := Parse([]byte(input)); err != nil || got.Checks[0].Steps[0].Run == "" {
			t.Fatalf("rejected valid Unicode %q: %#v %v", input, got, err)
		}
	}
	invalid := []string{
		strings.Replace(validConfig, `"run":"true"`, `"run":"\ud800"`, 1),
		strings.Replace(validConfig, `"run":"true"`, `"run":"\udc00"`, 1),
	}
	for _, input := range invalid {
		if got, err := Parse([]byte(input)); err == nil || !reflect.DeepEqual(got, model.Config{}) {
			t.Fatalf("accepted invalid Unicode %q as %#v (err=%v)", input, got, err)
		}
	}
	if _, err := Parse(append([]byte(validConfig), 0xff)); err == nil {
		t.Fatal("accepted invalid UTF-8")
	}
	input := strings.Replace(validConfig, `"run":"true"`, `"run":"\\uD800"`, 1)
	if got, err := Parse([]byte(input)); err != nil || got.Checks[0].Steps[0].Run != `\uD800` {
		t.Fatalf("escaped backslash-u was not preserved: %#v %v", got, err)
	}
}

func TestParsePreservesRun(t *testing.T) {
	run := "line 1\n\tline \"2\"\r\n$HOME,${VALUE}"
	encoded, _ := json.Marshal(run)
	input := strings.Replace(validConfig, `"run":"true"`, `"run":`+string(encoded), 1)
	got, err := Parse([]byte(input))
	if err != nil || got.Checks[0].Steps[0].Run != run {
		t.Fatalf("run changed after decode: %q %v", got.Checks[0].Steps[0].Run, err)
	}
}

func TestParseV1MigrationSkipsV2ShapeRequirements(t *testing.T) {
	for _, input := range []string{`{"schemaVersion":1,"ſteps":0}`, `{"schemaVersion":1,"workflow":null}`} {
		if _, err := Parse([]byte(input)); err == nil || !strings.Contains(err.Error(), "schemaVersion 1") {
			t.Fatalf("legacy payload did not reach migration error: %v", err)
		}
	}
}

func TestParseRejectsVersionAliasesBeforeMigration(t *testing.T) {
	for _, input := range []string{
		`{"schemaVersion":1,"SchemaVersion":2}`,
		`{"schemaVersion":1,"ſchemaVersion":2}`,
	} {
		got, err := Parse([]byte(input))
		if err == nil || !reflect.DeepEqual(got, model.Config{}) || strings.Contains(err.Error(), "schemaVersion 1") {
			t.Fatalf("accepted version alias as legacy migration: %#v %v", got, err)
		}
	}
}

func TestParseSizeAndDepth(t *testing.T) {
	room := model.MaxConfigBytes - len(validConfig)
	atLimit := strings.TrimSuffix(validConfig, "}") + strings.Repeat(" ", room) + "}"
	if len(atLimit) != model.MaxConfigBytes {
		t.Fatalf("test fixture is %d bytes, want %d", len(atLimit), model.MaxConfigBytes)
	}
	if _, err := Parse([]byte(atLimit)); err != nil {
		t.Fatalf("rejected config at byte limit: %v", err)
	}
	if got, err := Parse([]byte(atLimit + "a")); err == nil || !reflect.DeepEqual(got, model.Config{}) {
		t.Fatalf("accepted config over byte limit: %#v %v", got, err)
	}
	if got, err := Parse([]byte(`{"schemaVersion":3,"unused":{"x":0}}`)); err == nil || !reflect.DeepEqual(got, model.Config{}) || !strings.Contains(err.Error(), "unsupported config schemaVersion") {
		t.Fatalf("unsupported schema version was not rejected explicitly: %#v %v", got, err)
	}
	var large strings.Builder
	large.WriteString(`{"schemaVersion":1,"unused":{`)
	for i := 0; i < 5000; i++ {
		if i > 0 {
			large.WriteByte(',')
		}
		large.WriteString(`"member` + strconv.Itoa(i) + `":0`)
	}
	large.WriteString(`}}`)
	if _, err := Parse([]byte(large.String())); err == nil || !strings.Contains(err.Error(), "schemaVersion 1") {
		t.Fatalf("large unused v1 object did not reach migration: %v", err)
	}
	duplicate := strings.TrimSuffix(strings.TrimSuffix(large.String(), `}`), `}`) + `,"\u006dember4999":1}}`
	if got, err := Parse([]byte(duplicate)); err == nil || !reflect.DeepEqual(got, model.Config{}) || strings.Contains(err.Error(), "schemaVersion 1") {
		t.Fatalf("large unused decoded duplicate bypassed preflight: %#v %v", got, err)
	}

	nested := "0"
	for i := 0; i < model.MaxJSONDepth-1; i++ {
		nested = "[" + nested + "]"
	}
	within := `{"schemaVersion":1,"unused":` + nested + `}`
	if _, err := Parse([]byte(within)); err == nil || !strings.Contains(err.Error(), "schemaVersion 1") {
		t.Fatalf("rejected depth at limit before migration: %v", err)
	}
	nested += "]"
	nested = "[" + nested
	if got, err := Parse([]byte(`{"schemaVersion":1,"unused":` + nested + `}`)); err == nil || !reflect.DeepEqual(got, model.Config{}) || strings.Contains(err.Error(), "schemaVersion 1") {
		t.Fatalf("accepted depth beyond limit: %#v %v", got, err)
	}
}

func TestParseStructuredIssues(t *testing.T) {
	var ve *model.ValidationError
	_, err := Parse([]byte(strings.Replace(validConfig, `"runner":`, `"runner":`, 1)[:len(validConfig)-1] + `,"x\u001b[31m":1}`))
	if !errors.As(err, &ve) || ve.Issue.Code != "JSON_UNKNOWN_FIELD" || ve.Issue.Path != `/x\u001b[31m` {
		t.Fatalf("unknown field issue = %#v (%v)", ve, err)
	}
	if strings.Contains(err.Error(), "\x1b") {
		t.Fatalf("raw escape leaked: %q", err)
	}
	_, err = Parse([]byte(strings.Replace(validConfig, `"run":"true"`, `"run":1`, 1)))
	if !errors.As(err, &ve) || ve.Issue.Code != "JSON_TYPE" {
		t.Fatalf("type issue = %#v (%v)", ve, err)
	}
	_, err = Parse([]byte(strings.Replace(validConfig, `"run":"true"`, `"name":"","run":"true"`, 1)))
	if !errors.As(err, &ve) || ve.Issue.Code != "FIELD_INVALID" || ve.Issue.Path != "/checks/0/steps/0/name" {
		t.Fatalf("empty name issue = %#v (%v)", ve, err)
	}
}

func TestParseShapeIssuesCarryJSONPointers(t *testing.T) {
	var ve *model.ValidationError
	if _, err := Parse([]byte(validConfig)); err != nil {
		t.Fatalf("absent optional name rejected: %v", err)
	}
	missing := strings.Replace(validConfig, `,"run":"true"`, "", 1)
	_, err := Parse([]byte(missing))
	if !errors.As(err, &ve) || ve.Issue.Code != "FIELD_REQUIRED" || ve.Issue.Path != "/checks/0/steps/0/run" {
		t.Fatalf("missing run issue = %#v (%v)", ve, err)
	}
	_, err = Parse([]byte(strings.Replace(validConfig, `"run":"true"`, `"run":1`, 1)))
	if !errors.As(err, &ve) || ve.Issue.Code != "JSON_TYPE" || ve.Issue.Path != "/checks/0/steps/0/run" {
		t.Fatalf("wrong run issue = %#v (%v)", ve, err)
	}
	input := validConfig[:len(validConfig)-1] + `,"unknown\u202eKey":1}`
	_, err = Parse([]byte(input))
	if !errors.As(err, &ve) || ve.Issue.Code != "JSON_UNKNOWN_FIELD" || strings.ContainsRune(ve.Issue.Path, '\u202e') || !strings.Contains(ve.Issue.Path, `\u202e`) {
		t.Fatalf("format-control path = %#v (%v)", ve, err)
	}
}

func TestParseV1MigrationCarriesIssue(t *testing.T) {
	var ve *model.ValidationError
	_, err := Parse([]byte(`{"schemaVersion":1,"workflow":"ci.yml"}`))
	if !errors.As(err, &ve) || ve.Issue.Code != "SCHEMA_VERSION" || ve.Issue.Path != "/schemaVersion" || !strings.Contains(err.Error(), "schemaVersion 1") {
		t.Fatalf("migration issue = %#v (%v)", ve, err)
	}
}

func TestParseJobsElementTypesCarryJSONPointers(t *testing.T) {
	for _, value := range []string{"1", "null", "true", "{}", "[]"} {
		input := strings.Replace(validConfig, `"jobs":["verify"]`, `"jobs":[`+value+`]`, 1)
		got, err := Parse([]byte(input))
		var ve *model.ValidationError
		if err == nil || !reflect.DeepEqual(got, model.Config{}) || !errors.As(err, &ve) || ve.Issue.Code != "JSON_TYPE" || ve.Issue.Path != "/jobs/0" {
			t.Fatalf("jobs element %s: config=%#v issue=%#v err=%v", value, got, ve, err)
		}
	}
}

func TestParseRejectsWrongContainersWithTrustedPaths(t *testing.T) {
	secret := "SECRET_COMMAND"
	cases := []struct {
		name, input, path string
	}{
		{"runner array", strings.Replace(validConfig, `"runner":{"image":"alpine:3.22","architecture":"linux/arm64"}`, `"runner":["`+secret+`"]`, 1), "/runner"},
		{"runner null", strings.Replace(validConfig, `"runner":{"image":"alpine:3.22","architecture":"linux/arm64"}`, `"runner":null`, 1), "/runner"},
		{"checks object", strings.Replace(validConfig, `"checks":[{"id":"verify","steps":[{"id":"check","run":"true"}]}]`, `"checks":{"private":"`+secret+`"}`, 1), "/checks"},
		{"checks null", strings.Replace(validConfig, `"checks":[{"id":"verify","steps":[{"id":"check","run":"true"}]}]`, `"checks":null`, 1), "/checks"},
		{"check primitive", strings.Replace(validConfig, `"checks":[{"id":"verify","steps":[{"id":"check","run":"true"}]}]`, `"checks":["`+secret+`"]`, 1), "/checks/0"},
		{"check object steps", strings.Replace(validConfig, `"steps":[{"id":"check","run":"true"}]`, `"steps":{"private":"`+secret+`"}`, 1), "/checks/0/steps"},
		{"steps null", strings.Replace(validConfig, `"steps":[{"id":"check","run":"true"}]`, `"steps":null`, 1), "/checks/0/steps"},
		{"step array", strings.Replace(validConfig, `"steps":[{"id":"check","run":"true"}]`, `"steps":["`+secret+`"]`, 1), "/checks/0/steps/0"},
		{"step null", strings.Replace(validConfig, `"steps":[{"id":"check","run":"true"}]`, `"steps":[null]`, 1), "/checks/0/steps/0"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Parse([]byte(tc.input))
			var ve *model.ValidationError
			if err == nil || !reflect.DeepEqual(got, model.Config{}) || !errors.As(err, &ve) {
				t.Fatalf("config=%#v issue=%#v err=%v", got, ve, err)
			}
			if ve.Issue.Code != "JSON_TYPE" || ve.Issue.Path != tc.path {
				t.Fatalf("issue=%#v want JSON_TYPE at %s", ve.Issue, tc.path)
			}
			if strings.Contains(err.Error(), secret) {
				t.Fatalf("diagnostic leaked secret: %q", err)
			}
		})
	}
}

func TestParseIssuePathStopsAtUTF8Boundary(t *testing.T) {
	key := strings.Repeat("ก", 120)
	input := validConfig[:len(validConfig)-1] + `,"` + key + `":1}`
	_, err := Parse([]byte(input))
	var ve *model.ValidationError
	if !errors.As(err, &ve) || ve.Issue.Code != "JSON_UNKNOWN_FIELD" || len(ve.Issue.Path) > model.MaxIssuePathBytes || !utf8.ValidString(ve.Issue.Path) {
		t.Fatalf("issue=%#v err=%v", ve, err)
	}
}
