package workflow

import (
	"os"
	"path/filepath"
	"pitstop/internal/model"
	"strings"
	"testing"
)

func TestInspectJobsAndBlocksUnsupportedShapes(t *testing.T) {
	d := t.TempDir()
	p := filepath.Join(d, "ci.yml")
	y := []byte("name: CI\non: [pull_request]\njobs:\n  verify:\n    name: Verify\n    runs-on: ubuntu-latest\n    steps:\n      - run: echo ok\n  matrix:\n    runs-on: ubuntu-latest\n    strategy:\n      matrix:\n        go: [1,2]\n    steps: []\n")
	if err := os.WriteFile(p, y, 0600); err != nil {
		t.Fatal(err)
	}
	inv, err := Inspect(d, p)
	if err != nil {
		t.Fatal(err)
	}
	if len(inv.Jobs) != 2 || inv.Jobs[0].ID != "verify" {
		t.Fatalf("inventory=%#v", inv)
	}
	if inv.Jobs[1].CapabilityReasons == nil {
		t.Fatal("matrix should be blocked")
	}
}

func TestPlanDoesNotBroadenNeeds(t *testing.T) {
	inv := InventoryFixture([]string{"changes", "integration"})
	for i := range inv.Jobs {
		if inv.Jobs[i].ID == "integration" {
			inv.Jobs[i].Needs = []string{"changes"}
			inv.Jobs[i].If = "needs.changes.outputs.persistence == 'true'"
		}
	}
	cfg := modelConfig("integration")
	p, err := Plan("/repo", cfg, inv, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Jobs) != 1 || p.Jobs[0].Eligible {
		t.Fatalf("unexpected plan=%#v", p)
	}
}

func TestPrepareIntegrationRemovesOnlyJobGuards(t *testing.T) {
	d := t.TempDir()
	src := filepath.Join(d, "ci.yml")
	dst := filepath.Join(d, "prepared.yml")
	y := []byte("jobs:\n  changes:\n    runs-on: ubuntu-latest\n    steps: []\n  integration:\n    needs: changes\n    if: needs.changes.outputs.persistence == 'true'\n    runs-on: ubuntu-latest\n    steps:\n      - run: go test -race -tags integration ./...\n")
	if err := os.WriteFile(src, y, 0600); err != nil {
		t.Fatal(err)
	}
	a, err := PrepareIntegration(src, dst)
	if err != nil {
		t.Fatal(err)
	}
	if len(a) != 2 {
		t.Fatalf("adjustments=%#v", a)
	}
	got, err := os.ReadFile(dst)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) == string(y) || !containsText(string(got), "go test -race -tags integration ./...") {
		t.Fatalf("prepared=%s", got)
	}
}
func TestPrepareIntegrationFailsClosedOnShapeDrift(t *testing.T) {
	d := t.TempDir()
	src := filepath.Join(d, "ci.yml")
	if err := os.WriteFile(src, []byte("jobs:\n  integration:\n    needs: other\n    if: needs.other.outputs.persistence == 'true'\n    steps:\n      - run: go test -race -tags integration ./...\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := PrepareIntegration(src, filepath.Join(d, "out.yml")); err == nil {
		t.Fatal("expected adapter shape rejection")
	}
}
func TestInspectBlocksFalseStepsAndLocalSecrets(t *testing.T) {
	d := t.TempDir()
	if err := os.MkdirAll(filepath.Join(d, ".github/actions/check"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(d, ".github/actions/check/action.yml"), []byte("runs:\n  using: composite\n  steps:\n    - run: echo ${{ secrets.AUDIT_DUMMY }}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(d, "ci.yml")
	if err := os.WriteFile(p, []byte("jobs:\n  verify:\n    runs-on: ubuntu-latest\n    steps:\n      - if: false\n        run: echo no\n      - uses: ./.github/actions/check\n"), 0600); err != nil {
		t.Fatal(err)
	}
	inv, err := Inspect(d, p)
	if err != nil {
		t.Fatal(err)
	}
	if len(inv.Jobs[0].CapabilityReasons) < 2 {
		t.Fatalf("%#v", inv.Jobs[0])
	}
}
func containsText(s, sub string) bool { return strings.Contains(s, sub) }
func InventoryFixture(ids []string) model.Inventory {
	out := model.Inventory{}
	for _, id := range ids {
		out.Jobs = append(out.Jobs, model.JobInfo{ID: id, Name: id})
	}
	return out
}
func modelConfig(job string) model.Config {
	return model.Config{SchemaVersion: 1, Workflow: "ci.yml", Event: "pull_request", Jobs: []string{job}, Runner: model.RunnerConfig{Label: "ubuntu-latest", Image: "x", Architecture: "linux/arm64"}}
}
