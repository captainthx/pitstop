package report

import (
	"encoding/json"
	"fmt"
	"io"
	"pitstop/internal/model"
	"strings"
	"unicode"
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

// DisplayMessage makes untrusted human-facing text terminal-safe and bounded.
func DisplayMessage(s string) string { return display(s, model.MaxIssueMessageBytes) }

func display(s string, limit int) string {
	var b strings.Builder
	for _, r := range s {
		text := string(r)
		if r == 0x7f || unicode.IsControl(r) || unicode.In(r, unicode.Cf) {
			if r <= 0xffff {
				text = fmt.Sprintf("\\u%04x", r)
			} else {
				text = fmt.Sprintf("\\U%08x", r)
			}
		}
		if b.Len()+len(text) > limit {
			if b.Len()+len("…") <= limit {
				b.WriteString("…")
			}
			break
		}
		b.WriteString(text)
	}
	return b.String()
}

func displayPath(s string) string { return display(s, model.MaxIssuePathBytes) }

func Human(w io.Writer, e model.Envelope) error {
	if e.Error != nil {
		_, err := fmt.Fprintf(w, "pitstop: %s (%s)\n", DisplayMessage(e.Error.Message), DisplayMessage(e.Error.Code))
		if err != nil {
			return err
		}
		switch e.Result.(type) {
		case model.RunReport, model.ValidateResult, model.SkillInstallResult:
		default:
			return nil
		}
	}
	if r, ok := e.Result.(model.RunReport); ok {
		if _, err := fmt.Fprintf(w, "pitstop run %s: exit %d engine=%s selected=%s workflow=%s duration=%dms report=%s\n", DisplayMessage(r.RunID), e.ExitCode, DisplayMessage(r.Engine.Name), DisplayMessage(r.Coverage.Selected), DisplayMessage(r.Coverage.Workflow), r.DurationMs, displayPath(r.ReportPath)); err != nil {
			return err
		}
		for _, j := range r.Jobs {
			if _, err := fmt.Fprintf(w, "  %s: %s", DisplayMessage(j.ID), DisplayMessage(j.Status)); err != nil {
				return err
			}
			if j.ReasonCode != nil {
				if _, err := fmt.Fprintf(w, " (%s)", DisplayMessage(*j.ReasonCode)); err != nil {
					return err
				}
			}
			if j.Reason != nil {
				if _, err := fmt.Fprintf(w, " %s", DisplayMessage(*j.Reason)); err != nil {
					return err
				}
			}
			if _, err := fmt.Fprintf(w, " log=%s\n", displayPath(j.LogPath)); err != nil {
				return err
			}
		}
		for _, warning := range r.Warnings {
			if _, err := fmt.Fprintln(w, "warning:", DisplayMessage(warning)); err != nil {
				return err
			}
		}
		return nil
	}
	if r, ok := e.Result.(model.ValidateResult); ok {
		if _, err := fmt.Fprintf(w, "validate: valid=%t repo=%s config=%s architecture=%s checks=%d steps=%d\n", r.Valid, displayPath(r.Repo), displayPath(r.ConfigPath), DisplayMessage(r.Architecture), r.CheckCount, r.StepCount); err != nil {
			return err
		}
		for _, issue := range r.Issues {
			if _, err := fmt.Fprintf(w, "  %s %s: %s\n", DisplayMessage(issue.Code), displayPath(issue.Path), DisplayMessage(issue.Message)); err != nil {
				return err
			}
		}
		return nil
	}
	if r, ok := e.Result.(model.InitResult); ok {
		if _, err := fmt.Fprintf(w, "init: preset=%s architecture=%s config=%s dryRun=%t written=%t destinationExists=%t\n", DisplayMessage(r.Preset), DisplayMessage(r.Architecture), displayPath(r.ConfigPath), r.DryRun, r.Written, r.DestinationExists); err != nil {
			return err
		}
		for _, warning := range r.Warnings {
			if _, err := fmt.Fprintln(w, "warning:", DisplayMessage(warning)); err != nil {
				return err
			}
		}
		return nil
	}
	if r, ok := e.Result.(model.SkillInstallResult); ok {
		if _, err := fmt.Fprintf(w, "skill install: agent=%s scope=%s action=%s installed=%t target=%s version=%s dryRun=%t\n", DisplayMessage(r.Agent), DisplayMessage(r.Scope), DisplayMessage(r.Action), r.Installed, displayPath(r.TargetPath), DisplayMessage(r.BundleVersion), r.DryRun); err != nil {
			return err
		}
		for _, warning := range r.Warnings {
			if _, err := fmt.Fprintln(w, "warning:", DisplayMessage(warning)); err != nil {
				return err
			}
		}
		return nil
	}
	if r, ok := e.Result.(model.ListResult); ok {
		if _, err := fmt.Fprintf(w, "repo: %s\nchecks: %s\n", displayPath(r.Repo), displayPath(r.ConfigPath)); err != nil {
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
			if _, err := fmt.Fprintf(w, "  %s: %s", DisplayMessage(j.ID), DisplayMessage(status)); err != nil {
				return err
			}
			if len(j.Reasons) > 0 {
				if _, err := fmt.Fprintf(w, " (%s)", DisplayMessage(strings.Join(j.Reasons, "; "))); err != nil {
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
			if _, err := fmt.Fprintf(w, "  %s: %s %s\n", DisplayMessage(c.Name), DisplayMessage(c.Status), DisplayMessage(c.Message)); err != nil {
				return err
			}
		}
		return nil
	}
	_, err := fmt.Fprintf(w, "pitstop %s: exit %d\n", DisplayMessage(e.Command), e.ExitCode)
	return err
}
