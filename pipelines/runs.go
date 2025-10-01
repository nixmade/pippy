package pipelines

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/nixmade/pippy/audit"
	"github.com/nixmade/pippy/store"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"
)

func displayInputs(inputs map[string]string) string {
	var output []string
	for key, value := range inputs {
		output = append(output, fmt.Sprintf("%s=%s", key, value))
	}
	return strings.Join(output, ",")
}

func listPipelineRuns(pipelineRuns []*PipelineRun) {
	rows := [][]string{}
	for _, pipelineRun := range pipelineRuns {
		rows = append(rows, []string{pipelineRun.Created.Format(time.RFC3339), pipelineRun.Id, pipelineRun.State, pipelineRun.Updated.Sub(pipelineRun.Created).String(), displayInputs(pipelineRun.Inputs)})
	}

	sort.Slice(rows, func(i, j int) bool {
		leftTime, err := time.Parse(time.RFC3339, rows[i][0])
		if err != nil {
			return false
		}

		rightTime, err := time.Parse(time.RFC3339, rows[j][0])
		if err != nil {
			return false
		}

		return leftTime.After(rightTime)
	})

	re := lipgloss.NewRenderer(os.Stdout)

	var (
		// HeaderStyle is the lipgloss style used for the table headers.
		HeaderStyle = re.NewStyle().Foreground(purple).Bold(true).Align(lipgloss.Center)
		// CellStyle is the base lipgloss style used for the table rows.
		CellStyle = re.NewStyle().Padding(0, 1).Width(14)
		// OddRowStyle is the lipgloss style used for odd-numbered table rows.
		OddRowStyle = CellStyle.Foreground(bright)
		// EvenRowStyle is the lipgloss style used for even-numbered table rows.
		EvenRowStyle = CellStyle.Foreground(dim)
		// BorderStyle is the lipgloss style used for the table border.
		BorderStyle = lipgloss.NewStyle().Foreground(dim)
	)

	t := table.New().
		Width(120).
		Border(lipgloss.RoundedBorder()).
		BorderStyle(BorderStyle).
		Headers("TIME", "ID", "STATE", "RUN TIME", "INPUTS").
		Rows(rows...).
		StyleFunc(func(row, col int) lipgloss.Style {
			var style lipgloss.Style
			switch {
			case row == 0:
				style = HeaderStyle
			case row%2 == 0:
				style = EvenRowStyle
			default:
				style = OddRowStyle
			}

			if col == 0 {
				style = style.Width(18)
			}
			if col == 1 {
				style = style.Width(36)
			}
			if col == 2 {
				style = style.Width(8)
			}
			if col == 4 {
				style = style.Width(20)
			}
			return style
		})

	fmt.Println(t)
}

func showPipelineRun(name, id string, pipelineRun *PipelineRun) {
	s := currentStyle.Render(fmt.Sprintf("Pipeline %s with run id %s started at %s", name, id, pipelineRun.Created.String())) + "\n\n"

	for _, stage := range pipelineRun.Stages {
		var approvedBy string
		approval := stage.Metadata.Approval
		if approval.Name != "" || approval.Login != "" {
			approvedBy = fmt.Sprintf("%s(%s)", approval.Name, approval.Login)
		}
		if strings.EqualFold(stage.State, "Success") {
			s += checkMark.PaddingRight(1).Render(stage.Title) + " " + doneStyle.Render(stage.Completed.Sub(stage.Started).String())
			s += descriptionStyle.Faint(true).Render("\n    " + stage.Url)
			if approvedBy != "" {
				s += descriptionStyle.Faint(true).Render("\n    Approved by ") + doneStyle.Render(approvedBy)
			}
			s += "\n"
			continue
		} else if strings.EqualFold(stage.State, "InProgress") {
			s += bulletMark.Render() + " " + currentStyle.Render(stage.Title) + " " + currentStyle.Render(stage.Completed.Sub(stage.Started).String())
			s += descriptionStyle.Faint(true).Render("\n    " + stage.Url)
			if approvedBy != "" {
				s += descriptionStyle.Faint(true).Render("\n    Approved by ") + doneStyle.Render(approvedBy)
			}
			if stage.Rollback != nil {
				s += descriptionStyle.Faint(true).Render("\n    Rollback " + stage.Rollback.State + " " + stage.Rollback.Title)
				s += descriptionStyle.Faint(true).Render("\n    	" + stage.Rollback.Url)
			}
			s += "\n"
			continue
		} else if strings.EqualFold(stage.State, "Failed") {
			if stage.Rollback != nil {
				s += rollbackMark.Render() + " " + failedStyle.Render(stage.Title) + " " + failedStyle.Render(stage.Completed.Sub(stage.Started).String())
			} else {
				s += crossMark.Render() + " " + failedStyle.Render(stage.Title) + " " + failedStyle.Render(stage.Completed.Sub(stage.Started).String())
			}
			s += descriptionStyle.Faint(true).Render("\n    " + stage.Url)
			if approvedBy != "" {
				s += descriptionStyle.Faint(true).Render("\n    Approved by ") + doneStyle.Render(approvedBy)
			}
			if stage.Rollback != nil {
				s += warningStyle.Faint(true).Render("\n    Rollback " + stage.Rollback.State + " " + stage.Rollback.Title)
				s += warningStyle.Faint(true).Render("\n    	" + stage.Rollback.Url)
			}
			s += "\n"
			continue
		} else if strings.EqualFold(stage.State, "PendingApproval") {
			s += clockMark.Render() + " " + warningStyle.Render(stage.Name)
			if approvedBy != "" {
				s += descriptionStyle.Faint(true).Render("\n    Approved by ") + doneStyle.Render(approvedBy)
			}
			s += "\n"
			continue
		}
		s += bulletMark.Render() + " " + waitStyle.Render(stage.Name) + "\n"
	}

	if State(pipelineRun.State) == SUCCESS {
		s += "\n" + checkMark.Render() + " " + doneStyle.Render(fmt.Sprintf("Successfully completed running at %s", pipelineRun.Updated.String()))
	}

	if State(pipelineRun.State) == FAILED {
		s += "\n" + crossMark.Render() + " " + failedStyle.Render(fmt.Sprintf("Failed running pipeline at %s", pipelineRun.Updated.String()))
	}

	if State(pipelineRun.State) == ROLLBACK {
		s += "\n" + rollbackMark.Render() + " " + failedStyle.Render(fmt.Sprintf("Rollback complete for pipeline at %s", pipelineRun.Updated.String()))
	}

	if State(pipelineRun.State) == PENDING_APPROVAL {
		s += "\n" + clockMark.Render() + " " + warningStyle.Render(fmt.Sprintf("Pending approval for pipeline at %s", pipelineRun.Updated.String()))
	}

	if State(pipelineRun.State) == PAUSED {
		resource := map[string]string{"Pipeline": pipelineRun.PipelineName, "PipelineRun": pipelineRun.Id}
		latestAudit, err := audit.Latest(context.Background(), AUDIT_PAUSED, resource)
		if err == nil {
			s += "\n" + clockMark.Render() + " " + warningStyle.Render(fmt.Sprintf("Paused pipeline at %s by %s", latestAudit.Time.String(), fmt.Sprintf("%s(%s) ", latestAudit.Actor, latestAudit.Email)))
		}
	}

	fmt.Println(s)
}

func GetPipelineRuns(ctx context.Context, name string) ([]*PipelineRun, error) {
	return GetPipelineRunsN(ctx, name, -1)
}

func GetPipelineRunsN(ctx context.Context, name string, limit int64) ([]*PipelineRun, error) {
	dbStore, err := store.Get(ctx)
	if err != nil {
		return nil, err
	}
	defer func() {
		if closeErr := store.Close(dbStore); closeErr != nil {
			err = closeErr
		}
	}()

	pipelineRunKeyPrefix := PipelineRunPrefix
	if name != "" {
		pipelineRunKeyPrefix = fmt.Sprintf("%s%s/", PipelineRunPrefix, name)
	}
	var pipelineRuns []*PipelineRun
	pipelineRunItr := func(key any, value any) error {
		pipelineRun := &PipelineRun{}
		if err := json.Unmarshal([]byte(value.(string)), pipelineRun); err != nil {
			return err
		}
		pipelineRuns = append(pipelineRuns, pipelineRun)
		return nil
	}
	err = dbStore.SortedDescN(pipelineRunKeyPrefix, "$.created", limit, pipelineRunItr)
	if err != nil {
		return nil, err
	}

	return pipelineRuns, nil
}

func GetPipelineRunCount(ctx context.Context, name string) (uint64, error) {
	dbStore, err := store.Get(ctx)
	if err != nil {
		return 0, err
	}
	defer func() {
		if closeErr := store.Close(dbStore); closeErr != nil {
			err = closeErr
		}
	}()

	pipelineRunKeyPrefix := PipelineRunPrefix
	if name != "" {
		pipelineRunKeyPrefix = fmt.Sprintf("%s%s/", PipelineRunPrefix, name)
	}
	count, err := dbStore.Count(pipelineRunKeyPrefix)
	if err != nil {
		return 0, err
	}

	return count, nil
}

func GetPipelineRunCountByState(ctx context.Context, name string) (map[string]int64, error) {
	dbStore, err := store.Get(ctx)
	if err != nil {
		return nil, err
	}
	defer func() {
		if closeErr := store.Close(dbStore); closeErr != nil {
			err = closeErr
		}
	}()

	pipelineRunKeyPrefix := PipelineRunPrefix
	if name != "" {
		pipelineRunKeyPrefix = fmt.Sprintf("%s%s/", PipelineRunPrefix, name)
	}
	counts := make(map[string]int64)
	countsItr := func(key any, value any) error {
		counts[key.(string)] = value.(int64)
		return nil
	}
	err = dbStore.CountJsonPath(pipelineRunKeyPrefix, "$.state", countsItr)
	if err != nil {
		return nil, err
	}

	return counts, nil
}

func GetPipelineRun(ctx context.Context, name, id string) (*PipelineRun, error) {
	dbStore, err := store.Get(ctx)
	if err != nil {
		return nil, err
	}
	defer func() {
		if closeErr := store.Close(dbStore); closeErr != nil {
			err = closeErr
		}
	}()

	pipelineRunKey := fmt.Sprintf("%s%s/%s", PipelineRunPrefix, name, id)
	pipelineRun := &PipelineRun{}

	if err = dbStore.LoadJSON(pipelineRunKey, pipelineRun); err != nil {
		return nil, err
	}

	return pipelineRun, nil
}

func CancelPipelineRun(ctx context.Context, name, id string) error {
	pipelineRun, err := GetPipelineRun(ctx, name, id)
	if err != nil {
		if errors.Is(err, store.ErrKeyNotFound) {
			// if the pipeline run is found there is nothing to set
			return nil
		}
		return err
	}

	if pipelineRun.State != string(SUCCESS) && pipelineRun.State != string(FAILED) {
		pipelineRun.State = string(CANCELED)
		return savePipelineRun(ctx, pipelineRun)
	}

	return nil
}

func savePipelineRun(ctx context.Context, run *PipelineRun) error {
	dbStore, err := store.Get(ctx)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := store.Close(dbStore); closeErr != nil {
			err = closeErr
		}
	}()

	pipelineRunKey := fmt.Sprintf("%s%s/%s", PipelineRunPrefix, run.PipelineName, run.Id)
	return dbStore.SaveJSON(pipelineRunKey, run)
}

func ShowAllPipelineRuns(name string, limit int64) error {
	pipelineRuns, err := GetPipelineRunsN(context.Background(), name, limit)
	if err != nil {
		return err
	}

	listPipelineRuns(pipelineRuns)

	return nil
}

func ShowPipelineRun(name, id string) error {
	pipelineRun, err := GetPipelineRun(context.Background(), name, id)
	if err != nil {
		if errors.Is(err, store.ErrKeyNotFound) {
			s := "\n" + crossMark.PaddingRight(1).Render() +
				failedStyle.Render("pipeline run ") +
				warningStyle.Render(id) +
				failedStyle.Render(" for pipeline ") +
				warningStyle.Render(name) +
				failedStyle.Render(" not found\n")
			fmt.Println(s)
			return nil
		}
	}

	showPipelineRun(name, id, pipelineRun)

	return nil
}
