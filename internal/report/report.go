package report

import (
	"encoding/json"
	"fmt"
	"io"
	"pitstop/internal/model"
	"strings"
)

func ExitCode(jobs []model.JobResult, cancelled bool) int {
	if cancelled {
		return 130
	}
	for _, j := range jobs {
		if j.Status == "failed" {
			return 1
		}
	}
	if len(jobs) == 0 {
		return 2
	}
	for _, j := range jobs {
		if j.Status != "passed" {
			return 2
		}
	}
	return 0
}
func WriteEnvelope(w io.Writer, e model.Envelope) error {
	enc := json.NewEncoder(w)
	return enc.Encode(e)
}
func Human(w io.Writer, e model.Envelope) error {
	if e.Error != nil {
		_, err := fmt.Fprintf(w, "pitstop: %s (%s)\n", e.Error.Message, e.Error.Code)
		if _, ok := e.Result.(model.RunReport); !ok {
			if _, validate := e.Result.(model.ValidateResult); !validate || err != nil {
				return err
			}
		}
		if err != nil {
			return err
		}
	}
	if r, ok := e.Result.(model.RunReport); ok {
		if _, err := fmt.Fprintf(w, "pitstop run %s: exit %d engine=%s selected=%s workflow=%s duration=%dms report=%s\n", r.RunID, e.ExitCode, r.Engine.Name, r.Coverage.Selected, r.Coverage.Workflow, r.DurationMs, r.ReportPath); err != nil {
			return err
		}
		for _, j := range r.Jobs {
			if _, err := fmt.Fprintf(w, "  %s: %s", j.ID, j.Status); err != nil {
				return err
			}
			if j.ReasonCode != nil {
				if _, err := fmt.Fprintf(w, " (%s)", *j.ReasonCode); err != nil {
					return err
				}
			}
			if j.Reason != nil {
				if _, err := fmt.Fprintf(w, " %s", *j.Reason); err != nil {
					return err
				}
			}
			if _, err := fmt.Fprintf(w, " log=%s\n", j.LogPath); err != nil {
				return err
			}
		}
		for _, warning := range r.Warnings {
			if _, err := fmt.Fprintln(w, "warning:", warning); err != nil {
				return err
			}
		}
		return nil
	}
	if r, ok := e.Result.(model.ValidateResult); ok {
		if _, err := fmt.Fprintf(w, "validate: valid=%t repo=%s config=%s architecture=%s checks=%d steps=%d\n", r.Valid, r.Repo, r.ConfigPath, r.Architecture, r.CheckCount, r.StepCount); err != nil {
			return err
		}
		for _, issue := range r.Issues {
			if _, err := fmt.Fprintf(w, "  %s %s: %s\n", issue.Code, issue.Path, issue.Message); err != nil {
				return err
			}
		}
		return nil
	}
	if r, ok := e.Result.(model.ListResult); ok {
		if _, err := fmt.Fprintf(w, "repo: %s\nchecks: %s\n", r.Repo, r.ConfigPath); err != nil {
			return err
		}
		for _, j := range r.Jobs {
			status := "excluded"
			if j.Selected {
				status = "selected"
			}
			if !j.Eligible {
				status += " blocked"
			}
			if _, err := fmt.Fprintf(w, "  %s: %s", j.ID, status); err != nil {
				return err
			}
			if len(j.Reasons) > 0 {
				if _, err := fmt.Fprintf(w, " (%s)", strings.Join(j.Reasons, "; ")); err != nil {
					return err
				}
			}
			if _, err := fmt.Fprintln(w); err != nil {
				return err
			}
		}
		return nil
	}
	if r, ok := e.Result.(model.DoctorResult); ok {
		if _, err := fmt.Fprintf(w, "doctor: ready=%t\n", r.Ready); err != nil {
			return err
		}
		for _, c := range r.Checks {
			if _, err := fmt.Fprintf(w, "  %s: %s %s\n", c.Name, c.Status, c.Message); err != nil {
				return err
			}
		}
		return nil
	}
	_, err := fmt.Fprintf(w, "pitstop %s: exit %d\n", e.Command, e.ExitCode)
	return err
}
