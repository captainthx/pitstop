package model

import "testing"

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
