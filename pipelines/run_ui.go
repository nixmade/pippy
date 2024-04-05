package pipelines

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	currentStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("211"))
	doneStyle        = lipgloss.NewStyle().Foreground(lipgloss.Color("#00c468"))
	failedStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("#c4002a"))
	warningStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("#ffe57f"))
	waitStyle        = lipgloss.NewStyle().Foreground(lipgloss.Color("#808080"))
	descriptionStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#E6E6E6"))
	checkMark        = lipgloss.NewStyle().Foreground(lipgloss.Color("42")).SetString("✓")
	crossMark        = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#f44336", Dark: "#cc0000"}).SetString("x")
	bulletMark       = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#847A85", Dark: "#979797"}).SetString("•")
	clockMark        = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#ffe57f", Dark: "#ffcc00"}).SetString("⌛")
	rollbackMark     = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#f44336", Dark: "#cc0000"}).SetString("⎌")
)

type model struct {
	name        string
	runId       string
	spinner     spinner.Model
	stages      []string
	startedAt   string
	stageStatus *status
}

func initialModel(pipeline *Pipeline, stageStatus *status, startedAt string, runId string) model {
	var stages []string
	for _, stage := range pipeline.Stages {
		stages = append(stages, stage.Workflow.Name)
	}
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))
	return model{
		stages:      stages,
		name:        pipeline.Name,
		runId:       runId,
		spinner:     s,
		startedAt:   startedAt,
		stageStatus: stageStatus,
	}
}

func (m model) Init() tea.Cmd {
	return m.spinner.Tick
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		}
	default:
		switch m.stageStatus.GetState() {
		case SUCCESS, FAILED, PAUSED, LOCKED, ROLLBACK:
			return m, tea.Quit
		default:
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			return m, cmd
		}
	}

	return m, nil
}

func (m model) View() string {
	s := currentStyle.Render(fmt.Sprintf("Running pipeline %s using run id %s, started at %s", m.name, m.runId, m.startedAt)) + "\n\n"

	for i, stageName := range m.stages {
		if status := m.stageStatus.GetCache(getStageName(i, stageName)); status != nil {
			title := stageName
			if status.title != "" {
				title = status.title
			}
			if strings.EqualFold(status.state, "Success") {
				s += checkMark.PaddingRight(1).Render(title) + " " + doneStyle.Render(status.completed.Sub(status.started).String())
				s += descriptionStyle.Faint(true).Render("\n    " + status.runUrl)
				if status.approvedBy != "" {
					s += descriptionStyle.Faint(true).Render("\n    Approved by ") + doneStyle.Render(status.approvedBy)
				}
				s += "\n"
				continue
			} else if strings.EqualFold(status.state, "InProgress") {
				s += m.spinner.View() + " " + currentStyle.Render(title) + " " + currentStyle.Render(time.Now().UTC().Sub(status.started).String())
				s += descriptionStyle.Faint(true).Render("\n    " + status.runUrl)
				if status.approvedBy != "" {
					s += descriptionStyle.Faint(true).Render("\n    Approved by ") + doneStyle.Render(status.approvedBy)
				}
				if status.rollback != nil {
					rollbackTitle := stageName
					if status.rollback.title != "" {
						rollbackTitle = status.rollback.title
					}
					s += descriptionStyle.Faint(true).Render("\n    Rollback ") + status.rollback.state + " " + doneStyle.Render(rollbackTitle)
					s += descriptionStyle.Faint(true).Render("\n    	") + doneStyle.Render(status.rollback.runUrl)
				}
				s += "\n"
				continue
			} else if strings.EqualFold(status.state, "Failed") {
				if status.rollback != nil {
					s += rollbackMark.Render() + " " + failedStyle.Render(title) + " " + failedStyle.Render(status.completed.Sub(status.started).String())
				} else {
					s += crossMark.Render() + " " + failedStyle.Render(title) + " " + failedStyle.Render(status.completed.Sub(status.started).String())
				}
				s += descriptionStyle.Faint(true).Render("\n    " + status.runUrl)
				if status.approvedBy != "" {
					s += descriptionStyle.Faint(true).Render("\n    Approved by ") + doneStyle.Render(status.approvedBy)
				}
				if status.rollback != nil {
					s += warningStyle.Faint(true).Render("\n    Rollback " + status.rollback.state + " " + status.rollback.title)
					s += warningStyle.Faint(true).Render("\n    	" + status.rollback.runUrl)
				}
				s += "\n"
				continue
			} else if strings.EqualFold(status.state, "PendingApproval") {
				s += clockMark.Render() + " " + warningStyle.Render(stageName)
				if status.approvedBy != "" {
					s += descriptionStyle.Faint(true).Render("\n    Approved by ") + doneStyle.Render(status.approvedBy)
				}
				s += "\n"
				continue
			} else if strings.EqualFold(status.state, "ConcurrentError") {
				s += crossMark.Render() + " " + failedStyle.Render(title)
				s += failedStyle.Render("\n    Another pipeline run in progress, " + status.concurrentRunId)
				s += failedStyle.Render("\n     DANGER! optionally force this version using --force")
				s += "\n"
				continue
			}

		}
		s += bulletMark.Render() + " " + waitStyle.Render(stageName) + "\n"
	}

	if m.stageStatus.GetState() == SUCCESS {
		s += "\n" + checkMark.Render() + " " + doneStyle.Render(fmt.Sprintf("Successfully completed running pipeline %s with run id %s\n", m.name, m.runId))
	}

	if m.stageStatus.GetState() == FAILED {
		s += "\n" + crossMark.Render() + " " + failedStyle.Render(fmt.Sprintf("Failed running pipeline %s with run id %s\n", m.name, m.runId))
	}

	if m.stageStatus.GetState() == ROLLBACK {
		s += "\n" + rollbackMark.Render() + " " + failedStyle.Render(fmt.Sprintf("Rollback complete for pipeline %s with run id %s\n", m.name, m.runId))
	}

	if m.stageStatus.GetState() == PENDING_APPROVAL {
		s += "\n" + clockMark.Render() + " " + warningStyle.Render(fmt.Sprintf("Pending approval for pipeline %s with run id %s\n", m.name, m.runId))
	}

	if m.stageStatus.GetState() == PAUSED {
		s += "\n" + clockMark.Render() + " " + warningStyle.Render(fmt.Sprintf("Paused pipeline %s with run id %s\n", m.name, m.runId))
	}

	return s
}
