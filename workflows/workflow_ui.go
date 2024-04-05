package workflows

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
	"github.com/nixmade/pippy/github"
)

var (
	docStyle         = lipgloss.NewStyle().Margin(1, 2)
	currentStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("211"))
	doneStyle        = lipgloss.NewStyle().Foreground(lipgloss.Color("#00c468"))
	descriptionStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#E6E6E6"))
	failedStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("#c4002a"))
	checkMark        = lipgloss.NewStyle().Foreground(lipgloss.Color("42")).SetString("✓")
	crossMark        = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#f44336", Dark: "#cc0000"}).SetString("x")
)

type model struct {
	list list.Model
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
	case tea.WindowSizeMsg:
		h, v := docStyle.GetFrameSize()
		m.list.SetSize(msg.Width-h, msg.Height-v)
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m model) View() string {
	return docStyle.Render(m.list.View())
}

func Run(org, repo string, items []list.Item) error {
	m := model{list: list.New(items, list.NewDefaultDelegate(), 0, 0)}
	m.list.Title = fmt.Sprintf("Workflows for %s/%s", org, repo)

	p := tea.NewProgram(m, tea.WithAltScreen())

	if _, err := p.Run(); err != nil {
		fmt.Println("Error running program:", err)
		return err
	}

	return nil
}

func GetRepos(repoType string) ([]string, error) {
	repos, err := github.DefaultClient.ListRepos(repoType)
	if err != nil {
		return nil, err
	}

	var titles []string
	for _, repo := range repos {
		titles = append(titles, repo.Name)
	}

	return titles, nil
}

func RunValidateRepoWorkflows(c github.Client, repoType string) error {
	var orgRepos []string
	repos, err := GetRepos(repoType)
	if err != nil {
		return err
	}
	if err := huh.NewForm(
		huh.NewGroup(
			huh.NewMultiSelect[string]().
				Options(huh.NewOptions(repos...)...).
				Title("Choose single/multiple repo").
				Description("use spacebar to select, workflows for selected repos will be validated next").
				Value(&orgRepos),
		)).Run(); err != nil {
		return err
	}

	for _, orgRepo := range orgRepos {
		orgRepoSlice := strings.SplitN(orgRepo, "/", 2)
		org := orgRepoSlice[0]
		repo := orgRepoSlice[1]
		workflows, err := c.ListWorkflows(org, repo)
		if err != nil {
			return err
		}

		if len(workflows) <= 0 {
			fmt.Println(currentStyle.Render("\nNo workflows found in repo " + orgRepo + "\n"))
			continue
		}

		fmt.Println(currentStyle.Render("\nValidations for repo " + orgRepo + "\n"))
		for _, workflow := range workflows {
			if workflow.Path == "" {
				continue
			}
			if changes, _, err := c.ValidateWorkflow(org, repo, workflow.Path); err != nil {
				return err
			} else {
				if len(changes) > 0 {
					fmt.Println(crossMark.Render() + " " + failedStyle.Render(workflow.Name) + "(" + failedStyle.Render(workflow.Path) + descriptionStyle.Render(") failed validation, make changes in corresponding sections:\n"))
					for i, change := range changes {
						fmt.Println(currentStyle.Render("#" + strconv.Itoa(i+1) + "\n"))
						fmt.Println(descriptionStyle.Render(strings.Replace(change, "\"", "", -1)))
					}
				} else {
					fmt.Println(checkMark.Render() + " " + doneStyle.Render(workflow.Name) + "(" + doneStyle.Render(workflow.Path) + ") passed validation")
				}
			}
		}
		fmt.Println(currentStyle.Render("End of Validations for repo " + orgRepo + "\n"))
	}

	return nil
}
