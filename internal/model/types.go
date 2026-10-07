package model

import (
	"strings"
	"unicode"
)

// SchemaVersion is the version of the JSON envelope, not the project config.
const SchemaVersion = 1
const ConfigSchemaVersion = 2

type Error struct {
	Code, Message string
	Cause         error
}

func (e *Error) Error() string {
	if e.Cause != nil {
		return e.Code + ": " + e.Message + ": " + e.Cause.Error()
	}
	return e.Code + ": " + e.Message
}
func (e *Error) Unwrap() error { return e.Cause }
func Err(code, message string, cause error) *Error {
	return &Error{Code: code, Message: message, Cause: cause}
}

type Config struct {
	SchemaVersion int          `json:"schemaVersion"`
	Jobs          []string     `json:"jobs"`
	Runner        RunnerConfig `json:"runner"`
	Checks        []Check      `json:"checks"`
}
type RunnerConfig struct {
	Image        string `json:"image"`
	Architecture string `json:"architecture"`
}
type Check struct {
	ID    string      `json:"id"`
	Steps []CheckStep `json:"steps"`
}
type CheckStep struct {
	ID   string `json:"id"`
	Name string `json:"name,omitempty"`
	Run  string `json:"run"`
}

func (c Config) Validate() error {
	if c.SchemaVersion != ConfigSchemaVersion || len(c.Jobs) == 0 || len(c.Checks) == 0 || c.Runner.Image == "" {
		return Err("CONFIG_INVALID", "schemaVersion 2, jobs, runner.image, and checks are required", nil)
	}
	if c.Runner.Architecture != "linux/arm64" && c.Runner.Architecture != "linux/amd64" {
		return Err("CONFIG_INVALID", "runner.architecture must be linux/arm64 or linux/amd64", nil)
	}
	if invalidText(c.Runner.Image, true) || strings.HasPrefix(c.Runner.Image, "-") {
		return Err("CONFIG_INVALID", "runner.image must be a non-empty image reference without whitespace or control characters", nil)
	}
	seen := map[string]bool{}
	for _, j := range c.Jobs {
		if !validID(j) || seen[j] {
			return Err("CONFIG_INVALID", "jobs must contain unique valid IDs", nil)
		}
		seen[j] = true
	}
	checks := map[string]bool{}
	for _, check := range c.Checks {
		if !validID(check.ID) || checks[check.ID] {
			return Err("CONFIG_INVALID", "checks must contain unique valid IDs", nil)
		}
		checks[check.ID] = true
		if len(check.Steps) == 0 {
			return Err("CONFIG_INVALID", "each check must contain at least one step", nil)
		}
		steps := map[string]bool{}
		for _, step := range check.Steps {
			if !validID(step.ID) || steps[step.ID] {
				return Err("CONFIG_INVALID", "steps must contain unique valid IDs", nil)
			}
			if invalidText(step.Run, false) {
				return Err("CONFIG_INVALID", "step.run must be non-empty and contain no NUL", nil)
			}
			steps[step.ID] = true
		}
	}
	for _, j := range c.Jobs {
		if !checks[j] {
			return Err("CONFIG_INVALID", "jobs must refer to defined checks", nil)
		}
	}
	return nil
}
func invalidText(s string, allowWhitespace bool) bool {
	if s == "" || strings.IndexByte(s, 0) >= 0 {
		return true
	}
	if !allowWhitespace {
		return strings.TrimSpace(s) == ""
	}
	for _, r := range s {
		if unicode.IsSpace(r) || unicode.IsControl(r) {
			return true
		}
	}
	return false
}
func validID(s string) bool {
	if len(s) == 0 || len(s) > 64 || !((s[0] >= 'A' && s[0] <= 'Z') || (s[0] >= 'a' && s[0] <= 'z') || (s[0] >= '0' && s[0] <= '9')) {
		return false
	}
	for _, r := range s[1:] {
		if !((r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_' || r == '-') {
			return false
		}
	}
	return true
}

type Snapshot struct {
	Path               string   `json:"-"`
	ManifestPath       string   `json:"-"`
	SourceManifestPath string   `json:"-"`
	Head               string   `json:"head"`
	SnapshotSHA256     string   `json:"snapshotSha256"`
	Dirty              bool     `json:"dirty"`
	ExcludedPaths      []string `json:"excludedPaths"`
	CheckoutMode       string   `json:"checkoutMode"`
}
type EngineInfo struct {
	Name         string  `json:"name"`
	Version      string  `json:"version"`
	Architecture string  `json:"architecture"`
	Image        string  `json:"image"`
	ImageID      string  `json:"imageId,omitempty"`
	ImageDigest  *string `json:"imageDigest,omitempty"`
}
type StepResult struct {
	ID          string  `json:"id,omitempty"`
	Name        string  `json:"name,omitempty"`
	Stage       string  `json:"stage,omitempty"`
	Status      *string `json:"status"`
	DurationMs  *int64  `json:"durationMs"`
	EvidenceRef string  `json:"evidenceRef,omitempty"`
}
type JobResult struct {
	ID              string       `json:"id"`
	Status          string       `json:"status"`
	Provenance      string       `json:"provenance"`
	DurationMs      int64        `json:"durationMs"`
	EngineExitCode  *int         `json:"engineExitCode"`
	FailureCategory *string      `json:"failureCategory"`
	ReasonCode      *string      `json:"reasonCode"`
	Reason          *string      `json:"reason"`
	Steps           []StepResult `json:"steps"`
	LogPath         string       `json:"logPath"`
}
type Adjustment struct {
	JobID     string `json:"jobId"`
	Operation string `json:"operation"`
	YAMLPath  string `json:"yamlPath"`
	Before    string `json:"before"`
	After     string `json:"after"`
}
type Coverage struct {
	Selected string `json:"selectedCoverage"`
	Workflow string `json:"workflowCoverage"`
}
type RunReport struct {
	RunID  string      `json:"runId"`
	Repo   string      `json:"repo"`
	Source Snapshot    `json:"source"`
	Engine EngineInfo  `json:"engine"`
	Jobs   []JobResult `json:"jobs"`
	Coverage
	Adjustments []Adjustment `json:"adjustments"`
	Warnings    []string     `json:"warnings"`
	DurationMs  int64        `json:"durationMs"`
	ReportPath  string       `json:"reportPath"`
}
type ErrorInfo struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}
type Envelope struct {
	SchemaVersion int        `json:"schemaVersion"`
	Command       string     `json:"command"`
	ExitCode      int        `json:"exitCode"`
	Result        any        `json:"result,omitempty"`
	Error         *ErrorInfo `json:"error,omitempty"`
}
type DoctorCheck struct {
	Name       string `json:"name"`
	Status     string `json:"status"`
	ReasonCode string `json:"reasonCode,omitempty"`
	Message    string `json:"message"`
}
type DoctorResult struct {
	Repo   string        `json:"repo"`
	Checks []DoctorCheck `json:"checks"`
	Ready  bool          `json:"ready"`
}
type ListJob struct {
	ID       string   `json:"id"`
	Name     string   `json:"name"`
	Selected bool     `json:"selected"`
	Eligible bool     `json:"eligible"`
	Reasons  []string `json:"reasons"`
}
type ListResult struct {
	Repo       string    `json:"repo"`
	ConfigPath string    `json:"configPath"`
	Jobs       []ListJob `json:"jobs"`
}
type VersionResult struct {
	Version   string  `json:"version"`
	Commit    *string `json:"commit"`
	BuildTime *string `json:"buildTime"`
}

func Reason(s string) *string         { return &s }
func Category(s string) *string       { return &s }
func (e *Error) Is(target error) bool { t, ok := target.(*Error); return ok && e.Code == t.Code }
