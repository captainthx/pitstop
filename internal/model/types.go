package model

import (
	"errors"
	"fmt"
	"time"
)

const SchemaVersion = 1

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
	Workflow      string       `json:"workflow"`
	Event         string       `json:"event"`
	Jobs          []string     `json:"jobs"`
	Runner        RunnerConfig `json:"runner"`
	Adapter       string       `json:"adapter,omitempty"`
}
type RunnerConfig struct {
	Label        string `json:"label"`
	Image        string `json:"image"`
	Architecture string `json:"architecture"`
}

func (c Config) Validate() error {
	if c.SchemaVersion != SchemaVersion || c.Workflow == "" || c.Event != "pull_request" || len(c.Jobs) == 0 || c.Runner.Label == "" || c.Runner.Image == "" {
		return Err("CONFIG_INVALID", "schema, workflow, event, jobs, and runner fields are required", nil)
	}
	if c.Runner.Architecture != "linux/arm64" && c.Runner.Architecture != "linux/amd64" {
		return Err("CONFIG_INVALID", "runner.architecture must be linux/arm64 or linux/amd64", nil)
	}
	seen := map[string]bool{}
	for _, j := range c.Jobs {
		if j == "" || seen[j] {
			return Err("CONFIG_INVALID", "jobs must be non-empty and unique", nil)
		}
		seen[j] = true
	}
	return nil
}

type Options struct {
	Repo, Jobs               string
	JobList                  []string
	JSON                     bool
	Timeout                  time.Duration
	Architecture, ConfigPath string
	ForceIntegration         bool
}
type JobInfo struct {
	ID                string   `json:"id"`
	Name              string   `json:"name"`
	RunsOn            string   `json:"runsOn"`
	If                string   `json:"if,omitempty"`
	Needs             []string `json:"needs"`
	SecretReferences  []string `json:"secretReferences"`
	CapabilityReasons []string `json:"capabilityReasons"`
	Matrix            bool     `json:"matrix"`
	Reusable          bool     `json:"reusable"`
	Container         bool     `json:"container"`
	Services          bool     `json:"services"`
	ContinueOnError   bool     `json:"continueOnError"`
}
type Inventory struct {
	WorkflowPath string    `json:"workflowPath"`
	Jobs         []JobInfo `json:"jobs"`
}
type PlannedJob struct {
	ID       string   `json:"id"`
	Eligible bool     `json:"eligible"`
	Reasons  []string `json:"reasons"`
}
type Plan struct {
	Repo                  string       `json:"repo"`
	ConfigPath            string       `json:"-"`
	ExecutionWorkflowPath string       `json:"-"`
	Config                Config       `json:"config"`
	Inventory             Inventory    `json:"inventory"`
	Jobs                  []PlannedJob `json:"jobs"`
	ForceIntegration      bool         `json:"forceIntegration"`
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
	ImageDigest  *string `json:"imageDigest"`
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
	RunID       string       `json:"runId"`
	Repo        string       `json:"repo"`
	Source      Snapshot     `json:"source"`
	Engine      EngineInfo   `json:"engine"`
	Jobs        []JobResult  `json:"jobs"`
	Coverage    Coverage     `json:"coverage"`
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
	Repo         string    `json:"repo"`
	WorkflowPath string    `json:"workflowPath"`
	Jobs         []ListJob `json:"jobs"`
}
type VersionResult struct {
	Version   string  `json:"version"`
	Commit    *string `json:"commit"`
	BuildTime *string `json:"buildTime"`
}

func Reason(s string) *string         { return &s }
func Category(s string) *string       { return &s }
func (e *Error) Is(target error) bool { t, ok := target.(*Error); return ok && e.Code == t.Code }

var _ = errors.Is
var _ = fmt.Sprintf
