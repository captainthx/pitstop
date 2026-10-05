package model

import "testing"

func TestConfigValidationRejectsUnsupportedValues(t *testing.T) {
	for _, c := range []Config{
		{SchemaVersion: 2, Workflow: ".github/workflows/ci.yml", Event: "pull_request", Jobs: []string{"verify"}, Runner: RunnerConfig{Label: "ubuntu-latest", Image: "image", Architecture: "linux/arm64"}},
		{SchemaVersion: 1, Workflow: ".github/workflows/ci.yml", Event: "push", Jobs: []string{"verify"}, Runner: RunnerConfig{Label: "ubuntu-latest", Image: "image", Architecture: "linux/arm64"}},
		{SchemaVersion: 1, Workflow: ".github/workflows/ci.yml", Event: "pull_request", Jobs: []string{"verify", "verify"}, Runner: RunnerConfig{Label: "ubuntu-latest", Image: "image", Architecture: "linux/arm64"}},
	} {
		if err := c.Validate(); err == nil {
			t.Fatalf("expected invalid config: %#v", c)
		}
	}
}
