package workflow

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"go.yaml.in/yaml/v3"
	"pitstop/internal/model"
)

func Inspect(repo, path string) (model.Inventory, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return model.Inventory{}, model.Err("WORKFLOW_UNSUPPORTED", "cannot read workflow", err)
	}
	var root yaml.Node
	if err := yaml.Unmarshal(b, &root); err != nil {
		return model.Inventory{}, model.Err("WORKFLOW_UNSUPPORTED", "invalid workflow YAML", err)
	}
	if len(root.Content) == 0 {
		return model.Inventory{}, model.Err("WORKFLOW_UNSUPPORTED", "workflow document is empty", nil)
	}
	mapNode := root.Content[0]
	if mapNode == nil || mapNode.Kind != yaml.MappingNode {
		return model.Inventory{}, model.Err("WORKFLOW_UNSUPPORTED", "workflow document must be a mapping", nil)
	}
	jobs := lookup(mapNode, "jobs")
	if jobs == nil || jobs.Kind != yaml.MappingNode {
		return model.Inventory{}, model.Err("WORKFLOW_UNSUPPORTED", "workflow has no jobs mapping", nil)
	}
	inv := model.Inventory{WorkflowPath: path, Jobs: []model.JobInfo{}}
	ids := map[string]bool{}
	for i := 0; i < len(jobs.Content); i += 2 {
		id := jobs.Content[i].Value
		if ids[id] {
			return model.Inventory{}, model.Err("WORKFLOW_UNSUPPORTED", "duplicate job id: "+id, nil)
		}
		if !validJobID(id) {
			return model.Inventory{}, model.Err("WORKFLOW_UNSUPPORTED", "invalid job id: "+id, nil)
		}
		ids[id] = true
		n := jobs.Content[i+1]
		if n.Kind != yaml.MappingNode {
			continue
		}
		j := model.JobInfo{ID: id, Name: id, Needs: []string{}}
		if v := lookup(n, "name"); v != nil {
			j.Name = v.Value
		}
		if v := lookup(n, "runs-on"); v != nil {
			j.RunsOn = v.Value
		}
		if j.RunsOn == "" {
			j.CapabilityReasons = append(j.CapabilityReasons, "runner is missing")
		} else if strings.Contains(j.RunsOn, "${{") {
			j.CapabilityReasons = append(j.CapabilityReasons, "dynamic runner")
		}
		if v := lookup(n, "if"); v != nil {
			j.If = v.Value
		}
		if v := lookup(n, "needs"); v != nil {
			if v.Kind == yaml.ScalarNode {
				j.Needs = []string{v.Value}
			} else if v.Kind == yaml.SequenceNode {
				for _, x := range v.Content {
					j.Needs = append(j.Needs, x.Value)
				}
			} else {
				j.CapabilityReasons = append(j.CapabilityReasons, "dynamic needs")
			}
		}
		j.Matrix = lookup(n, "strategy") != nil
		j.Container = lookup(n, "container") != nil
		j.Services = lookup(n, "services") != nil
		j.ContinueOnError = lookup(n, "continue-on-error") != nil
		if lookup(n, "uses") != nil {
			j.Reusable = true
			j.CapabilityReasons = append(j.CapabilityReasons, "reusable workflows are unsupported")
		}
		if steps := lookup(n, "steps"); steps == nil || steps.Kind != yaml.SequenceNode || len(steps.Content) == 0 || !hasRunnableStep(steps) {
			j.CapabilityReasons = append(j.CapabilityReasons, "job has no unconditional executable steps")
		}
		if containsSecret(n) {
			j.SecretReferences = append(j.SecretReferences, "job")
		}
		if hasFalseCondition(n) {
			j.CapabilityReasons = append(j.CapabilityReasons, "conditional steps are unsupported")
		}
		for _, reason := range localActionReasons(repo, localActions(n)) {
			if strings.HasPrefix(reason, "secret:") {
				j.SecretReferences = append(j.SecretReferences, strings.TrimPrefix(reason, "secret:"))
			} else {
				j.CapabilityReasons = append(j.CapabilityReasons, reason)
			}
		}
		if top := lookup(mapNode, "env"); top != nil && containsSecret(top) {
			j.SecretReferences = append(j.SecretReferences, "workflow.env")
		}
		if j.Matrix {
			j.CapabilityReasons = append(j.CapabilityReasons, "matrix jobs are unsupported")
		}
		if j.Container {
			j.CapabilityReasons = append(j.CapabilityReasons, "job containers are unsupported")
		}
		if j.Services {
			j.CapabilityReasons = append(j.CapabilityReasons, "job services are unsupported")
		}
		if j.If != "" {
			j.CapabilityReasons = append(j.CapabilityReasons, "job conditions are unsupported")
		}
		if len(j.Needs) > 0 {
			j.CapabilityReasons = append(j.CapabilityReasons, "dependency-bearing jobs require adapter")
		}
		if j.ContinueOnError {
			j.CapabilityReasons = append(j.CapabilityReasons, "continue-on-error is unsupported")
		}
		if len(j.SecretReferences) > 0 {
			j.CapabilityReasons = append(j.CapabilityReasons, "secret references require credentials")
		}
		inv.Jobs = append(inv.Jobs, j)
	}
	known := map[string]bool{}
	for _, j := range inv.Jobs {
		known[j.ID] = true
	}
	for i := range inv.Jobs {
		for _, need := range inv.Jobs[i].Needs {
			if !known[need] {
				inv.Jobs[i].CapabilityReasons = append(inv.Jobs[i].CapabilityReasons, "missing dependency: "+need)
			}
		}
	}
	state := map[string]int{}
	var visit func(string)
	visit = func(id string) {
		state[id] = 1
		for i := range inv.Jobs {
			if inv.Jobs[i].ID != id {
				continue
			}
			for _, n := range inv.Jobs[i].Needs {
				if state[n] == 1 {
					inv.Jobs[i].CapabilityReasons = append(inv.Jobs[i].CapabilityReasons, "dependency cycle")
				}
				if state[n] == 0 && known[n] {
					visit(n)
				}
			}
		}
		state[id] = 2
	}
	for _, j := range inv.Jobs {
		if state[j.ID] == 0 {
			visit(j.ID)
		}
	}
	return inv, nil
}

var jobIDPattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_-]*$`)

func validJobID(id string) bool { return jobIDPattern.MatchString(id) }
func hasRunnableStep(n *yaml.Node) bool {
	if n == nil {
		return false
	}
	for _, step := range n.Content {
		if step.Kind != yaml.MappingNode {
			continue
		}
		if v := lookup(step, "if"); v != nil && strings.TrimSpace(v.Value) != "" && strings.TrimSpace(v.Value) != "true" {
			continue
		}
		if lookup(step, "run") != nil || lookup(step, "uses") != nil {
			return true
		}
	}
	return false
}
func lookup(n *yaml.Node, key string) *yaml.Node {
	if n == nil || n.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(n.Content); i += 2 {
		if n.Content[i].Value == key {
			return n.Content[i+1]
		}
	}
	return nil
}
func containsSecret(n *yaml.Node) bool {
	if n == nil {
		return false
	}
	if strings.Contains(n.Value, "secrets.") || strings.Contains(n.Value, "secrets[") || strings.Contains(n.Value, "secrets [") {
		return true
	}
	for _, c := range n.Content {
		if containsSecret(c) {
			return true
		}
	}
	return false
}
func hasFalseCondition(n *yaml.Node) bool {
	if n == nil {
		return false
	}
	if n.Kind == yaml.MappingNode {
		for i := 0; i+1 < len(n.Content); i += 2 {
			if n.Content[i].Value == "if" && strings.TrimSpace(n.Content[i+1].Value) == "false" {
				return true
			}
		}
	}
	for _, c := range n.Content {
		if hasFalseCondition(c) {
			return true
		}
	}
	return false
}
func localActions(n *yaml.Node) []string {
	var out []string
	var walk func(*yaml.Node)
	walk = func(x *yaml.Node) {
		if x.Kind == yaml.MappingNode {
			for i := 0; i+1 < len(x.Content); i += 2 {
				if x.Content[i].Value == "uses" && strings.HasPrefix(x.Content[i+1].Value, "./") {
					out = append(out, x.Content[i+1].Value)
				}
			}
		}
		for _, c := range x.Content {
			walk(c)
		}
	}
	walk(n)
	return out
}
func localActionReasons(repo string, refs []string) []string {
	var out []string
	seen := map[string]bool{}
	stack := map[string]bool{}
	root, _ := filepath.EvalSymlinks(repo)
	var visit func(string, string)
	visit = func(ref, baseDir string) {
		base := filepath.Join(baseDir, strings.TrimPrefix(ref, "./"))
		real, e := filepath.EvalSymlinks(base)
		rel, relErr := filepath.Rel(root, real)
		if e != nil || relErr != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			out = append(out, "local action path escapes or is missing: "+ref)
			return
		}
		if stack[real] {
			out = append(out, "local action cycle: "+ref)
			return
		}
		if seen[real] {
			return
		}
		seen[real] = true
		stack[real] = true
		defer delete(stack, real)
		var doc yaml.Node
		var b []byte
		for _, name := range []string{"action.yml", "action.yaml"} {
			if bb, ee := os.ReadFile(filepath.Join(real, name)); ee == nil {
				b = bb
				break
			}
		}
		if len(b) == 0 {
			out = append(out, "local action metadata is missing: "+ref)
			return
		}
		if yaml.Unmarshal(b, &doc) != nil {
			out = append(out, "local action metadata is invalid: "+ref)
			return
		}
		if containsSecret(&doc) {
			out = append(out, "secret:local composite "+ref)
		}
		for _, nested := range localActions(&doc) {
			visit(nested, real)
		}
	}
	for _, r := range refs {
		visit(r, repo)
	}
	return out
}
func Plan(repo string, cfg model.Config, inv model.Inventory, force bool) (model.Plan, error) {
	selected := cfg.Jobs
	if len(selected) == 0 {
		return model.Plan{}, model.Err("CONFIG_INVALID", "no jobs configured", nil)
	}
	p := model.Plan{Repo: repo, Config: cfg, Inventory: inv, Jobs: []model.PlannedJob{}, ExecutionWorkflowPath: inv.WorkflowPath, ForceIntegration: force}
	known := map[string]bool{}
	for _, j := range inv.Jobs {
		known[j.ID] = true
	}
	for _, id := range selected {
		if !known[id] {
			return p, model.Err("CONFIG_INVALID", "configured job not found: "+id, nil)
		}
		for _, j := range inv.Jobs {
			if j.ID != id {
				continue
			}
			reasons := append([]string{}, j.CapabilityReasons...)
			if j.RunsOn != "" && j.RunsOn != cfg.Runner.Label {
				reasons = append(reasons, "runner does not match configured label")
			}
			if len(j.Needs) > 0 && len(reasons) == 0 {
				reasons = append(reasons, "dependency-bearing jobs require adapter")
			}
			if j.If != "" && len(reasons) == 0 {
				reasons = append(reasons, "job conditions are unsupported")
			}
			pj := model.PlannedJob{ID: id, Eligible: len(reasons) == 0, Reasons: reasons}
			approved := len(reasons) == 2 && containsReason(reasons, "dependency-bearing jobs require adapter") && containsReason(reasons, "job conditions are unsupported")
			if force && id == "integration" && cfg.Adapter == "recruit100-service-integration-v1" && approved {
				pj.Eligible = true
				pj.Reasons = nil
			}
			p.Jobs = append(p.Jobs, pj)
		}
	}
	return p, nil
}
func containsReason(xs []string, want string) bool {
	for _, x := range xs {
		if x == want {
			return true
		}
	}
	return false
}
func PrepareIntegration(source, destination string) ([]model.Adjustment, error) {
	b, err := os.ReadFile(source)
	if err != nil {
		return nil, model.Err("ADAPTER_SHAPE_CHANGED", "cannot read integration workflow", err)
	}
	var root yaml.Node
	if err := yaml.Unmarshal(b, &root); err != nil {
		return nil, model.Err("ADAPTER_SHAPE_CHANGED", "invalid integration workflow", err)
	}
	if len(root.Content) == 0 || root.Content[0] == nil || root.Content[0].Kind != yaml.MappingNode {
		return nil, model.Err("ADAPTER_SHAPE_CHANGED", "empty integration workflow", nil)
	}
	jobs := lookup(root.Content[0], "jobs")
	integration := lookup(jobs, "integration")
	if integration == nil || integration.Kind != yaml.MappingNode {
		return nil, model.Err("ADAPTER_SHAPE_CHANGED", "integration job is missing", nil)
	}
	needs := lookup(integration, "needs")
	cond := lookup(integration, "if")
	needsOK := needs != nil && ((needs.Kind == yaml.ScalarNode && needs.Value == "changes") || (needs.Kind == yaml.SequenceNode && len(needs.Content) == 1 && needs.Content[0].Value == "changes"))
	condValue := ""
	if cond != nil {
		condValue = strings.TrimSpace(cond.Value)
		condValue = strings.TrimSpace(strings.TrimPrefix(strings.TrimSuffix(condValue, "}}"), "${{"))
	}
	condOK := condValue == "needs.changes.outputs.persistence == 'true'"
	if !needsOK || !condOK || !containsRunCommand(integration, "go test -race -tags integration ./...") {
		return nil, model.Err("ADAPTER_SHAPE_CHANGED", fmt.Sprintf("integration shape changed in %s", source), nil)
	}
	var adjustments []model.Adjustment
	for _, key := range []string{"needs", "if"} {
		for i := 0; i+1 < len(integration.Content); i += 2 {
			if integration.Content[i].Value == key {
				v := integration.Content[i+1]
				adjustments = append(adjustments, model.Adjustment{JobID: "integration", Operation: "remove", YAMLPath: "jobs.integration." + key, Before: v.Value, After: ""})
				integration.Content = append(integration.Content[:i], integration.Content[i+2:]...)
				break
			}
		}
	}
	enc, err := yaml.Marshal(&root)
	if err != nil {
		return nil, model.Err("ADAPTER_SHAPE_CHANGED", "cannot encode prepared workflow", err)
	}
	if err := os.WriteFile(destination, enc, 0600); err != nil {
		return nil, err
	}
	return adjustments, nil
}
func containsRunCommand(n *yaml.Node, want string) bool {
	if n == nil {
		return false
	}
	if n.Kind == yaml.ScalarNode && strings.TrimSpace(n.Value) == want {
		return true
	}
	for _, c := range n.Content {
		if containsRunCommand(c, want) {
			return true
		}
	}
	return false
}
