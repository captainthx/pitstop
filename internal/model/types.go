package model

import (
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

// SchemaVersion is the version of the JSON envelope, not the project config.
const SchemaVersion = 1
const ConfigSchemaVersion = 2
const MaxConfigBytes = 1048576
const MaxJSONDepth = 16
const MaxJobs = 128
const MaxChecks = 128
const MaxStepsPerCheck = 128
const MaxTotalSteps = 2048
const MaxRunBytes = 65536
const MaxNameBytes = 256
const MaxImageBytes = 1024
const MaxIssueMessageBytes = 512
const MaxIssuePathBytes = 256

type ConfigIssue struct {
	Code    string `json:"code"`
	Path    string `json:"path"`
	Message string `json:"message"`
}

type ValidationError struct{ Issue ConfigIssue }

func (e *ValidationError) Error() string { return e.Issue.Message }

func issue(code, path, message string) error {
	return Err("CONFIG_INVALID", message, &ValidationError{Issue: ConfigIssue{Code: code, Path: path, Message: message}})
}

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
	if c.SchemaVersion != ConfigSchemaVersion {
		return issue("SCHEMA_VERSION", "/schemaVersion", "schemaVersion must be 2")
	}
	if len(c.Jobs) == 0 {
		return issue("FIELD_REQUIRED", "/jobs", "jobs is required")
	}
	if len(c.Jobs) > MaxJobs {
		return issue("LIMIT_EXCEEDED", "/jobs", "jobs exceeds the configured limit")
	}
	if len(c.Checks) == 0 {
		return issue("FIELD_REQUIRED", "/checks", "checks is required")
	}
	if len(c.Checks) > MaxChecks {
		return issue("LIMIT_EXCEEDED", "/checks", "checks exceeds the configured limit")
	}
	if c.Runner.Image == "" {
		return issue("FIELD_REQUIRED", "/runner/image", "runner.image is required")
	}
	if c.Runner.Architecture != "linux/arm64" && c.Runner.Architecture != "linux/amd64" {
		return issue("FIELD_INVALID", "/runner/architecture", "runner.architecture is unsupported")
	}
	if len(c.Runner.Image) > MaxImageBytes || strings.HasPrefix(c.Runner.Image, "-") || !validImage(c.Runner.Image) {
		return issue("FIELD_INVALID", "/runner/image", "runner.image is invalid")
	}
	seen := map[string]bool{}
	for i, j := range c.Jobs {
		path := "/jobs/" + strconv.Itoa(i)
		if !validID(j) {
			return issue("ID_INVALID", path, "job ID is invalid")
		}
		if seen[j] {
			return issue("ID_DUPLICATE", path, "job IDs must be unique")
		}
		seen[j] = true
	}
	checks := map[string]bool{}
	totalSteps := 0
	for i, check := range c.Checks {
		checkPath := "/checks/" + strconv.Itoa(i)
		if !validID(check.ID) {
			return issue("ID_INVALID", checkPath+"/id", "check ID is invalid")
		}
		if checks[check.ID] {
			return issue("ID_DUPLICATE", checkPath+"/id", "check IDs must be unique")
		}
		checks[check.ID] = true
		if len(check.Steps) == 0 {
			return issue("FIELD_REQUIRED", checkPath+"/steps", "each check requires a step")
		}
		if len(check.Steps) > MaxStepsPerCheck {
			return issue("LIMIT_EXCEEDED", checkPath+"/steps", "steps per check exceeds the configured limit")
		}
		totalSteps += len(check.Steps)
		if totalSteps > MaxTotalSteps {
			return issue("LIMIT_EXCEEDED", checkPath+"/steps", "total steps exceeds the configured limit")
		}
		steps := map[string]bool{}
		for j, step := range check.Steps {
			stepPath := checkPath + "/steps/" + strconv.Itoa(j)
			if !validID(step.ID) {
				return issue("ID_INVALID", stepPath+"/id", "step ID is invalid")
			}
			if steps[step.ID] {
				return issue("ID_DUPLICATE", stepPath+"/id", "step IDs must be unique")
			}
			if step.Name != "" {
				if len(step.Name) > MaxNameBytes || strings.TrimSpace(step.Name) == "" || hasUnsafeNameRune(step.Name) {
					return issue("FIELD_INVALID", stepPath+"/name", "step.name is invalid")
				}
			}
			if invalidRun(step.Run) {
				return issue("FIELD_INVALID", stepPath+"/run", "step.run is invalid")
			}
			steps[step.ID] = true
		}
	}
	for i, j := range c.Jobs {
		if !checks[j] {
			return issue("SELECTION_INVALID", "/jobs/"+strconv.Itoa(i), "job must refer to a defined check")
		}
	}
	return nil
}
func invalidRun(s string) bool {
	if s == "" || !utf8.ValidString(s) || len(s) > MaxRunBytes || strings.TrimSpace(s) == "" {
		return true
	}
	for _, r := range s {
		if unicode.IsControl(r) && r != '\n' && r != '\r' && r != '\t' {
			return true
		}
	}
	return false
}
func hasUnsafeNameRune(s string) bool {
	if !utf8.ValidString(s) {
		return true
	}
	for _, r := range s {
		if unicode.IsControl(r) || unicode.In(r, unicode.Cf) {
			return true
		}
	}
	return false
}
func validImage(s string) bool {
	for _, r := range s {
		if !(r >= 'A' && r <= 'Z' || r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || strings.ContainsRune("._:/@-", r)) {
			return false
		}
	}
	return s != ""
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
type ValidateResult struct {
	Repo         string        `json:"repo"`
	ConfigPath   string        `json:"configPath"`
	Valid        bool          `json:"valid"`
	SelectedJobs []string      `json:"selectedJobs"`
	Architecture string        `json:"architecture"`
	CheckCount   int           `json:"checkCount"`
	StepCount    int           `json:"stepCount"`
	Issues       []ConfigIssue `json:"issues"`
}
type InitResult struct {
	Repo              string   `json:"repo"`
	ConfigPath        string   `json:"configPath"`
	Preset            string   `json:"preset"`
	Architecture      string   `json:"architecture"`
	Config            Config   `json:"config"`
	DryRun            bool     `json:"dryRun"`
	Written           bool     `json:"written"`
	DestinationExists bool     `json:"destinationExists"`
	Warnings          []string `json:"warnings"`
}
type VersionResult struct {
	Version   string  `json:"version"`
	Commit    *string `json:"commit"`
	BuildTime *string `json:"buildTime"`
}

func Reason(s string) *string         { return &s }
func Category(s string) *string       { return &s }
func (e *Error) Is(target error) bool { t, ok := target.(*Error); return ok && e.Code == t.Code }
