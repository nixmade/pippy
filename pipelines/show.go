package pipelines

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"

	"github.com/nixmade/pippy/store"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"
)

const (
	purple = lipgloss.Color("#929292")
	bright = lipgloss.Color("#FDFF90")
	dim    = lipgloss.Color("#97AD64")
)

func showPipeline(pipeline *Pipeline) {
	rows := [][]string{}
	for i, stage := range pipeline.Stages {
		approval := "NO"
		if stage.Approval {
			approval = "YES"
			if pipeline.Locked {
				approval = "LOCKED"
			}
		}
		ignore := "NO"
		rollback := ""
		if stage.Monitor.Workflow.Ignore {
			ignore = "YES"
		}
		if stage.Monitor.Workflow.Rollback {
			rollback = "WORKFLOW"
		}
		datadog := "NO"
		if stage.Monitor.Datadog != nil {
			datadog = "YES"
			if stage.Monitor.Datadog.Rollback {
				if rollback == "" {
					rollback = "DATADOG"
				} else {
					rollback += ",DATADOG"
				}
			}
		}

		if rollback == "" {
			rollback = "NO"
		}

		rows = append(rows, []string{strconv.Itoa(i + 1), stage.Repo, stage.Workflow.Name, stage.Workflow.Url, approval, ignore, datadog, rollback})
	}

	re := lipgloss.NewRenderer(os.Stdout)

	var (
		// HeaderStyle is the lipgloss style used for the table headers.
		HeaderStyle = re.NewStyle().Foreground(purple).Bold(true).Align(lipgloss.Center)
		// CellStyle is the base lipgloss style used for the table rows.
		CellStyle = re.NewStyle().Padding(0, 1).Width(14)
		// OddRowStyle is the lipgloss style used for odd-numbered table rows.
		OddRowStyle = CellStyle.Copy().Foreground(bright)
		// EvenRowStyle is the lipgloss style used for even-numbered table rows.
		EvenRowStyle = CellStyle.Copy().Foreground(dim)
		// BorderStyle is the lipgloss style used for the table border.
		BorderStyle = lipgloss.NewStyle().Foreground(dim)
	)

	t := table.New().
		Width(120).
		Border(lipgloss.RoundedBorder()).
		BorderStyle(BorderStyle).
		Headers("#", "REPO", "WORKFLOW", "URL", "REQUIRES APPROVAL", "IGNORE WORKFLOW FAILURES", "DATADOG MONITORING", "ROLLBACK").
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
				style = style.Width(2)
			}
			if col == 1 {
				style = style.Width(8)
			}
			if col == 2 {
				style = style.Width(10)
			}
			if col == 3 {
				style = style.Width(16)
			}
			if col == 4 {
				style = style.Width(20)
			}
			if col == 5 {
				style = style.Width(26)
			}
			if col == 6 {
				style = style.Width(20)
			}
			if col == 7 {
				style = style.Width(10)
			}

			return style.AlignHorizontal(lipgloss.Center)
		})

	fmt.Println(t)
}

func ShowPipeline(name string) error {
	pipeline, err := GetPipeline(context.Background(), name)
	if err != nil {
		if errors.Is(err, store.ErrKeyNotFound) {
			s := "\n" + crossMark.PaddingRight(1).Render() +
				failedStyle.Render("pipeline ") +
				warningStyle.Render(name) +
				failedStyle.Render(" not found\n")
			fmt.Println(s)
			return nil
		}
		return err
	}

	showPipeline(pipeline)

	return nil
}

func listPipeline(pipelines []*Pipeline) error {
	rows := [][]string{}
	for i, pipeline := range pipelines {
		locked := "NO"
		if pipeline.Locked {
			locked = "YES"
		}
		runs, err := GetPipelineRunCountByState(context.Background(), pipeline.Name)
		if err != nil {
			return err
		}
		runCount := 0
		for _, value := range runs {
			runCount += int(value)
		}
		rows = append(rows, []string{strconv.Itoa(i + 1), pipeline.Name, strconv.Itoa(len(pipeline.Stages)), strconv.Itoa(runCount), locked})
	}

	re := lipgloss.NewRenderer(os.Stdout)

	var (
		// HeaderStyle is the lipgloss style used for the table headers.
		HeaderStyle = re.NewStyle().Foreground(purple).Bold(true).Align(lipgloss.Center)
		// CellStyle is the base lipgloss style used for the table rows.
		CellStyle = re.NewStyle().Padding(0, 1).Width(14)
		// OddRowStyle is the lipgloss style used for odd-numbered table rows.
		OddRowStyle = CellStyle.Copy().Foreground(bright)
		// EvenRowStyle is the lipgloss style used for even-numbered table rows.
		EvenRowStyle = CellStyle.Copy().Foreground(dim)
		// BorderStyle is the lipgloss style used for the table border.
		BorderStyle = lipgloss.NewStyle().Foreground(dim)
	)

	t := table.New().
		Width(120).
		Border(lipgloss.RoundedBorder()).
		BorderStyle(BorderStyle).
		Headers("#", "NAME", "STAGES", "RUNS", "APPROVALS LOCKED").
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
				style = style.Width(2)
			}
			if col == 1 {
				style = style.Width(48)
			}
			if col == 2 {
				style = style.Width(10)
			}
			if col == 3 {
				style = style.Width(10)
			}
			if col == 4 {
				style = style.Width(10)
			}
			return style.AlignHorizontal(lipgloss.Center)
		})

	fmt.Println(t)
	return nil
}

func ShowAllPipelines() error {
	pipelines, err := ListPipelines(context.Background())
	if err != nil {
		return err
	}

	return listPipeline(pipelines)
}
