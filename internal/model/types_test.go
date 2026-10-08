package model

import (
	"errors"
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestDockerConfigValidation(t *testing.T) {
	valid := func() Config {
		return Config{SchemaVersion: 2, Jobs: []string{"verify"}, Runner: RunnerConfig{Image: "node:22-bookworm", Architecture: "linux/arm64"}, Checks: []Check{{ID: "verify", Steps: []CheckStep{{ID: "check", Run: "npm run check"}}}}}
	}
	if err := valid().Validate(); err != nil {
		t.Fatal(err)
	}
	for _, mutate := range []func(*Config){
		func(c *Config) { c.Runner.Image = "--privileged" }, func(c *Config) { c.Runner.Image = "image\nother" }, func(c *Config) { c.Runner.Architecture = "darwin/arm64" },
		func(c *Config) { c.Checks[0].ID = "../escape" }, func(c *Config) { c.Checks[0].Steps[0].ID = "a/b" }, func(c *Config) { c.Checks[0].Steps[0].Run = " " },
		func(c *Config) { c.Checks[0].Steps[0].Run = "echo\x00x" }, func(c *Config) { c.Checks[0].Steps = nil }, func(c *Config) { c.Checks = append(c.Checks, c.Checks[0]) },
		func(c *Config) { c.Checks[0].Steps = append(c.Checks[0].Steps, c.Checks[0].Steps[0]) }, func(c *Config) { c.Jobs = []string{"missing"} }, func(c *Config) { c.Jobs = []string{"verify", "verify"} },
	} {
		c := valid()
		mutate(&c)
		if c.Validate() == nil {
			t.Fatalf("accepted %#v", c)
		}
	}
}

func TestConfigLimitsAndNames(t *testing.T) {
	base := func() Config {
		return Config{SchemaVersion: 2, Jobs: []string{"verify"}, Runner: RunnerConfig{Image: "node:22-bookworm", Architecture: "linux/arm64"}, Checks: []Check{{ID: "verify", Steps: []CheckStep{{ID: "check", Run: "true"}}}}}
	}
	valid := base()
	valid.Jobs = make([]string, MaxJobs)
	valid.Checks = make([]Check, MaxChecks)
	for i := range valid.Jobs {
		valid.Jobs[i] = "job" + strings.Repeat("x", 1) + string(rune('A'+i%26))
	}
	for i := range valid.Checks {
		valid.Checks[i] = Check{ID: "check" + string(rune('A'+i%26)) + strings.Repeat("x", i/26), Steps: []CheckStep{{ID: "step", Run: "true"}}}
	}
	valid.Jobs = make([]string, MaxChecks)
	for i := range valid.Jobs {
		valid.Jobs[i] = valid.Checks[i].ID
	}
	if err := valid.Validate(); err != nil {
		t.Fatalf("max jobs/checks rejected: %v", err)
	}
	tooMany := base()
	tooMany.Jobs = []string{"verify"}
	for i := 1; i < MaxJobs+1; i++ {
		tooMany.Jobs = append(tooMany.Jobs, "j"+strings.Repeat("x", i))
	}
	if err := tooMany.Validate(); err == nil {
		t.Fatal("jobs over limit accepted")
	}
	tooMany = base()
	for i := 1; i < MaxChecks+1; i++ {
		tooMany.Checks = append(tooMany.Checks, Check{ID: "c" + strings.Repeat("x", i), Steps: []CheckStep{{ID: "s", Run: "true"}}})
	}
	if err := tooMany.Validate(); err == nil {
		t.Fatal("checks over limit accepted")
	}
	steps := base()
	steps.Checks[0].Steps = make([]CheckStep, MaxStepsPerCheck)
	for i := range steps.Checks[0].Steps {
		steps.Checks[0].Steps[i] = CheckStep{ID: "s" + strconv.Itoa(i), Run: "true"}
	}
	if err := steps.Validate(); err != nil {
		t.Fatalf("max steps rejected: %v", err)
	}
	steps.Checks[0].Steps = append(steps.Checks[0].Steps, CheckStep{ID: "overflow", Run: "true"})
	if err := steps.Validate(); err == nil {
		t.Fatal("steps over limit accepted")
	}
	for _, run := range []string{"\x01", "\x0b", "\x1f", "\x7f", "echo \x00 secret"} {
		c := base()
		c.Checks[0].Steps[0].Run = run
		if err := c.Validate(); err == nil {
			t.Fatalf("control run accepted: %q", run)
		}
	}
	for _, run := range []string{"echo\nnext", "echo\rnext", "echo\tnext"} {
		c := base()
		c.Checks[0].Steps[0].Run = run
		if err := c.Validate(); err != nil {
			t.Fatalf("legitimate shell syntax rejected: %q: %v", run, err)
		}
	}
	for _, name := range []string{"สวัสดี"} {
		c := base()
		c.Checks[0].Steps[0].Name = name
		if err := c.Validate(); err != nil {
			t.Fatalf("Thai name rejected: %v", err)
		}
	}
	for _, name := range []string{"x\x1b[31m", "x\u200e"} {
		c := base()
		c.Checks[0].Steps[0].Name = name
		if err := c.Validate(); err == nil {
			t.Fatalf("unsafe name accepted: %q", name)
		}
	}
	longRun := base()
	longRun.Checks[0].Steps[0].Run = strings.Repeat("a", MaxRunBytes)
	if err := longRun.Validate(); err != nil {
		t.Fatalf("max run rejected: %v", err)
	}
	longRun.Checks[0].Steps[0].Run += "a"
	if err := longRun.Validate(); err == nil {
		t.Fatal("run over limit accepted")
	}
	if !utf8.ValidString(longRun.Checks[0].Steps[0].Run) {
		t.Fatal("test fixture invalid UTF-8")
	}
}

func TestConfigImageBounds(t *testing.T) {
	valid := func(image string) Config {
		return Config{SchemaVersion: 2, Jobs: []string{"verify"}, Runner: RunnerConfig{Image: image, Architecture: "linux/amd64"}, Checks: []Check{{ID: "verify", Steps: []CheckStep{{ID: "check", Run: "true"}}}}}
	}
	for _, image := range []string{"localhost:5000/team/checks:dev", "node:22-bookworm", "registry.example/team/checks@sha256:" + strings.Repeat("a", 64), strings.Repeat("a", MaxImageBytes)} {
		if err := valid(image).Validate(); err != nil {
			t.Fatalf("valid image rejected: %q: %v", image, err)
		}
	}
	for _, image := range []string{"--privileged", " image", "image ", "image\nother", "repo/$TAG", "repo/тест", strings.Repeat("a", MaxImageBytes+1)} {
		if err := valid(image).Validate(); err == nil {
			t.Fatalf("invalid image accepted: %q", image)
		}
	}
}

func TestValidationDiagnosticsNoContent(t *testing.T) {
	secret := "TOPSECRET-echo-payload"
	c := Config{SchemaVersion: 2, Jobs: []string{"verify"}, Runner: RunnerConfig{Image: "node:22-bookworm", Architecture: "linux/arm64"}, Checks: []Check{{ID: "verify", Steps: []CheckStep{{ID: "check", Run: secret}}}}}
	c.Checks[0].Steps[0].Run += "\x00tail"
	var ve *ValidationError
	err := c.Validate()
	if !errors.As(err, &ve) || ve.Issue.Path != "/checks/0/steps/0/run" {
		t.Fatalf("missing trusted validation issue: %#v", err)
	}
	if c.Checks[0].Steps[0].Run == "" || strings.Contains(err.Error(), secret) || strings.Contains(err.Error(), "\x1b") {
		t.Fatalf("diagnostic leaked content: %q", err)
	}
}

func TestConfigTotalStepsAndNameBoundaries(t *testing.T) {
	c := Config{SchemaVersion: 2, Runner: RunnerConfig{Image: "node:22-bookworm", Architecture: "linux/arm64"}}
	for i := 0; i < 16; i++ {
		id := "check" + strconv.Itoa(i)
		c.Jobs = append(c.Jobs, id)
		check := Check{ID: id}
		for j := 0; j < MaxStepsPerCheck; j++ {
			check.Steps = append(check.Steps, CheckStep{ID: "step" + strconv.Itoa(j), Run: "true"})
		}
		c.Checks = append(c.Checks, check)
	}
	if err := c.Validate(); err != nil {
		t.Fatalf("2048 steps rejected: %v", err)
	}
	over := c
	over.Jobs = append(append([]string(nil), c.Jobs...), "overflow")
	over.Checks = append(append([]Check(nil), c.Checks...), Check{ID: "overflow", Steps: []CheckStep{{ID: "step", Run: "true"}}})
	if err := over.Validate(); err == nil {
		t.Fatal("2049 steps accepted")
	}
	name := c
	name.Checks = []Check{{ID: "verify", Steps: []CheckStep{{ID: "step", Name: strings.Repeat("a", MaxNameBytes), Run: "true"}}}}
	name.Jobs = []string{"verify"}
	if err := name.Validate(); err != nil {
		t.Fatalf("256-byte name rejected: %v", err)
	}
	name.Checks[0].Steps[0].Name += "a"
	if err := name.Validate(); err == nil {
		t.Fatal("257-byte name accepted")
	}
	name.Checks[0].Steps[0].Name = ""
	if err := name.Validate(); err != nil {
		t.Fatalf("absent name representation rejected: %v", err)
	}
}
