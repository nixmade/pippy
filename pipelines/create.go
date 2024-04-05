package pipelines

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/nixmade/pippy/github"

	"github.com/nixmade/pippy/store"

	"github.com/charmbracelet/huh"
)

type Pipeline struct {
	Name string `json:"name"`
	//GroupStages []GroupStage `json:"group_stages"`
	Stages []Stage `json:"stages"`
	Locked bool    `json:"locked"`
}

// type GroupStage struct {
// 	Name   string  `json:"name"`
// 	Stages []Stage `json:"stages"`
// }

type DatadogInfo struct {
	Monitors       []string `json:"monitors"`
	Site           string   `json:"site"`
	ApiKey         string   `json:"api_key"`
	ApplicationKey string   `json:"application_key"`
	Rollback       bool     `json:"rollback"`
}

type WorkflowInfo struct {
	Ignore   bool `json:"ignore"`
	Rollback bool `json:"rollback"`
}

type MonitorInfo struct {
	// Monitor workflow state
	Workflow WorkflowInfo `json:"workflow,omitempty"`
	Datadog  *DatadogInfo `json:"datadog,omitempty"`
}

type Stage struct {
	Repo     string            `json:"repo"`
	Workflow github.Workflow   `json:"workflow"`
	Approval bool              `json:"approval"`
	Monitor  MonitorInfo       `json:"monitor,omitempty"`
	Input    map[string]string `json:"input,omitempty"`
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

func GetWorkflows(orgRepo string) ([]github.Workflow, error) {
	orgRepoSlice := strings.SplitN(orgRepo, "/", 2)
	return github.DefaultClient.ListWorkflows(orgRepoSlice[0], orgRepoSlice[1])
}

func ValidateWorkflow(orgRepo string, workflow github.Workflow) error {
	orgRepoSlice := strings.SplitN(orgRepo, "/", 2)
	changes, _, err := github.DefaultClient.ValidateWorkflow(orgRepoSlice[0], orgRepoSlice[1], workflow.Path)
	if err != nil {
		return nil
	}
	if len(changes) > 0 {
		return fmt.Errorf("this workflow is not pippy ready, please validate using pippy workflow validate")
	}
	return nil
}

func getWorkflowTitles(workflows []github.Workflow) []string {
	var titles []string
	for _, workflow := range workflows {
		titles = append(titles, workflow.Name)
	}

	return titles
}

func ListPipelines(ctx context.Context) ([]*Pipeline, error) {
	dbStore, err := store.Get(ctx)
	if err != nil {
		return nil, err
	}
	defer store.Close(dbStore)

	var pipelines []*Pipeline
	pipelineItr := func(key any, value any) error {
		pipeline := &Pipeline{}
		if err := json.Unmarshal([]byte(value.(string)), pipeline); err != nil {
			return err
		}
		pipelines = append(pipelines, pipeline)
		return nil
	}
	err = dbStore.LoadValues(PipelinePrefix, pipelineItr)
	if err != nil {
		return nil, err
	}

	return pipelines, nil
}

func GetPipelineCount(ctx context.Context) (uint64, error) {
	dbStore, err := store.Get(ctx)
	if err != nil {
		return 0, err
	}
	defer store.Close(dbStore)

	count, err := dbStore.Count(PipelinePrefix)
	if err != nil {
		return 0, err
	}

	return count, nil
}

func GetPipeline(ctx context.Context, name string) (*Pipeline, error) {
	dbStore, err := store.Get(ctx)
	if err != nil {
		return nil, err
	}
	defer store.Close(dbStore)

	pipeline := &Pipeline{}

	if err := dbStore.LoadJSON(PipelinePrefix+name, pipeline); err != nil {
		return nil, err
	}

	return pipeline, nil
}

func SavePipeline(ctx context.Context, pipeline *Pipeline) error {
	dbStore, err := store.Get(ctx)
	if err != nil {
		return err
	}
	defer store.Close(dbStore)

	return dbStore.SaveJSON(PipelinePrefix+pipeline.Name, pipeline)
}

func CreatePipeline(name, repoType string) error {
	if _, err := GetPipeline(context.Background(), name); err == nil {
		return fmt.Errorf("Pipeline %s already exists use pipeline show command", name)
	}

	pipeline := Pipeline{Name: name}

	repos, err := GetRepos(repoType)
	if err != nil {
		return err
	}

	workflowCache := make(map[string][]github.Workflow)

	for {
		stage := Stage{}
		if err := huh.NewForm(
			huh.NewGroup(
				huh.NewNote().
					Title(fmt.Sprintf("Add a new Stage for Pipeline %s", name)).
					Description(fmt.Sprintf("STAGE %d\n", len(pipeline.Stages)+1)),
				huh.NewSelect[string]().
					Options(huh.NewOptions(repos...)...).
					Title("Choose a repo").
					Description("Support public repos only, workflows will be queried in next step").
					Validate(func(t string) error {
						return nil
					}).
					Value(&stage.Repo),
			)).Run(); err != nil {
			return err
		}

		workflows, ok := workflowCache[stage.Repo]
		if !ok {
			workflows, err = GetWorkflows(stage.Repo)
			if err != nil {
				return nil
			}

			workflowCache[stage.Repo] = workflows
		}

		var workflowName string

		if err := huh.NewSelect[string]().
			Options(huh.NewOptions(getWorkflowTitles(workflows)...)...).
			Title("Choose a workflow").
			Description("Make sure that the workflow support similar inputs across a pipeline").
			Validate(func(t string) error {
				for _, workflow := range workflows {
					if strings.EqualFold(workflow.Name, t) {
						return ValidateWorkflow(stage.Repo, workflow)
					}
				}
				return fmt.Errorf("workflow not found in the list")
			}).
			Value(&workflowName).Run(); err != nil {
			return err
		}

		var datadogMonitoring bool
		var input string
		if err := huh.NewForm(
			huh.NewGroup(
				huh.NewInput().
					Title("Provide input override? eg: key=value,key1=value1").
					Value(&input),
				huh.NewConfirm().
					Title("Approval required?").
					Affirmative("Yes!").
					Negative("No.").
					Value(&stage.Approval),
				huh.NewConfirm().
					Title("Ignore any failures?").
					Affirmative("Yes!").
					Negative("No.").
					Value(&stage.Monitor.Workflow.Ignore),
				huh.NewConfirm().
					Title("Rollback on workflow failures?").
					Affirmative("Yes!").
					Negative("No.").
					Value(&stage.Monitor.Workflow.Rollback),
				huh.NewConfirm().
					Title("Datadog monitoring?").
					Affirmative("Yes!").
					Negative("No.").
					Value(&datadogMonitoring),
			)).Run(); err != nil {
			return err
		}

		if datadogMonitoring {
			var monitorIds, apiKey, applicationKey string
			var rollback bool
			site := "datadoghq.com"
			if err := huh.NewForm(
				huh.NewGroup(
					huh.NewNote().
						Title(fmt.Sprintf("Datadog setup %s", name)).
						Description("provide monitor ids, api and application key"),
					huh.NewInput().
						Title("Monitor ids? eg: monitor_id1,monitor_id2").
						Value(&monitorIds).
						Validate(func(t string) error {
							if t == "" {
								return errors.New("monitor Ids cannot be empty")
							}
							return nil
						}),
					huh.NewInput().
						Title("Site").
						Value(&site).
						Placeholder("datadoghq.com"),
					huh.NewInput().
						Title("API Key").
						Value(&apiKey).
						Validate(func(t string) error {
							if t == "" {
								return errors.New("API Key cannot be empty")
							}
							return nil
						}),
					huh.NewInput().
						Title("Application Key").
						Value(&applicationKey).
						Validate(func(t string) error {
							if t == "" {
								return errors.New("application Key cannot be empty")
							}
							return nil
						}),
					huh.NewConfirm().
						Title("Rollback on failure?").
						Affirmative("Yes!").
						Negative("No.").
						Value(&rollback),
				)).Run(); err != nil {
				return err
			}
			ids := strings.Split(monitorIds, ",")
			stage.Monitor.Datadog = &DatadogInfo{Monitors: ids, Site: site, ApiKey: apiKey, ApplicationKey: applicationKey, Rollback: rollback}
		}

		stage.Input = make(map[string]string)
		inputs := strings.Split(input, ",")
		for _, str := range inputs {
			keyVal := strings.SplitN(str, "=", 2)
			if len(keyVal) == 2 {
				stage.Input[keyVal[0]] = keyVal[1]
			}
		}

		for _, workflow := range workflows {
			if strings.EqualFold(workflow.Name, workflowName) {
				stage.Workflow = workflow
			}
		}

		pipeline.Stages = append(pipeline.Stages, stage)

		confirm := true
		if err := huh.NewConfirm().
			Title("Do you want to specify more stages?").
			Affirmative("Yes!").
			Negative("No.").
			Value(&confirm).Run(); err != nil {
			return err
		}

		if !confirm {
			break
		}
	}

	showPipeline(&pipeline)

	return SavePipeline(context.Background(), &pipeline)
}
