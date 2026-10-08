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
		_, err := fmt.Fprintf(w, "pitstop run %s: exit %d engine=%s selected=%s workflow=%s duration=%dms report=%s\n", r.RunID, e.ExitCode, r.Engine.Name, r.Coverage.Selected, r.Coverage.Workflow, r.DurationMs, r.ReportPath)
		for _, j := range r.Jobs {
			fmt.Fprintf(w, "  %s: %s", j.ID, j.Status)
			if j.ReasonCode != nil {
				fmt.Fprintf(w, " (%s)", *j.ReasonCode)
			}
			if j.Reason != nil {
				fmt.Fprintf(w, " %s", *j.Reason)
			}
			fmt.Fprintf(w, " log=%s\n", j.LogPath)
		}
		for _, warning := range r.Warnings {
			fmt.Fprintln(w, "warning:", warning)
		}
		return err
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
		fmt.Fprintf(w, "repo: %s\nchecks: %s\n", r.Repo, r.ConfigPath)
		for _, j := range r.Jobs {
			status := "excluded"
			if j.Selected {
				status = "selected"
			}
			if !j.Eligible {
				status += " blocked"
			}
			fmt.Fprintf(w, "  %s: %s", j.ID, status)
			if len(j.Reasons) > 0 {
				fmt.Fprintf(w, " (%s)", strings.Join(j.Reasons, "; "))
			}
			fmt.Fprintln(w)
		}
		return nil
	}
	if r, ok := e.Result.(model.DoctorResult); ok {
		fmt.Fprintf(w, "doctor: ready=%t\n", r.Ready)
		for _, c := range r.Checks {
			fmt.Fprintf(w, "  %s: %s %s\n", c.Name, c.Status, c.Message)
		}
		return nil
	}
	_, err := fmt.Fprintf(w, "pitstop %s: exit %d\n", e.Command, e.ExitCode)
	return err
}
