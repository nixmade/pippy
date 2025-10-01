package github

import (
	"context"
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/google/go-github/v75/github"
)

type Workflow struct {
	Name, Url, State, Path string
	Id                     int64
}

func (i Workflow) Title() string       { return i.Name }
func (i Workflow) Description() string { return i.Url }
func (i Workflow) FilterValue() string { return i.Name }

func (g *Github) GetWorkflow(org, repo string, id int64) (*Workflow, error) {
	client, err := g.New()
	if err != nil {
		return nil, err
	}

	workflow, _, err := client.Actions.GetWorkflowByID(context.Background(), org, repo, id)
	if err != nil {
		return nil, err
	}

	return &Workflow{
		Name:  workflow.GetName(),
		Url:   workflow.GetHTMLURL(),
		Id:    workflow.GetID(),
		State: workflow.GetState(),
		Path:  workflow.GetPath(),
	}, nil
}

func (g *Github) ListWorkflows(org, repo string) ([]Workflow, error) {
	client, err := g.New()
	if err != nil {
		return nil, err
	}

	opt := &github.ListOptions{}
	workflows, _, err := client.Actions.ListWorkflows(context.Background(), org, repo, opt)
	if err != nil {
		return nil, err
	}

	var workflowItems []Workflow

	for i := 0; i < len(workflows.Workflows); i++ {
		workflowItems = append(workflowItems, Workflow{
			Name:  workflows.Workflows[i].GetName(),
			Url:   workflows.Workflows[i].GetHTMLURL(),
			Id:    workflows.Workflows[i].GetID(),
			State: workflows.Workflows[i].GetState(),
			Path:  workflows.Workflows[i].GetPath(),
		})
	}

	return workflowItems, nil
}

func (g *Github) ValidateWorkflow(org, repo, path string) ([]string, map[string]string, error) {
	client, err := g.New()
	if err != nil {
		return nil, nil, err
	}

	opt := &github.RepositoryContentGetOptions{}
	fileContent, _, _, err := client.Repositories.GetContents(context.Background(), org, repo, path, opt)
	if err != nil {
		return nil, nil, err
	}

	content, err := fileContent.GetContent()
	if err != nil {
		return nil, nil, err
	}

	workflowsDef := make(map[string]interface{})
	err = yaml.Unmarshal([]byte(content), workflowsDef)
	if err != nil {
		return nil, nil, err
	}

	var workflowName string
	var workflowRunName string
	//workflowDispatchInputs := make(map[string]string)

	if value, ok := workflowsDef["name"]; ok {
		workflowName = value.(string)
	}

	if value, ok := workflowsDef["run-name"]; ok {
		workflowRunName = value.(string)
	}

	var requiredChanges []string
	workflowInputs := make(map[string]string)

	if onDispatches, ok := workflowsDef["on"]; ok {
		if dispatches, ok := onDispatches.(map[string]interface{}); ok {
			if workflowDispatch, ok := dispatches["workflow_dispatch"]; ok {
				if workflowDispatchInputs, ok := workflowDispatch.(map[string]interface{}); ok {
					if inputs, ok := workflowDispatchInputs["inputs"]; ok {
						newInputs := make(map[string]interface{})
						for key, value := range inputs.(map[string]interface{}) {
							inputType := value.(map[string]interface{})["type"]
							newInputs[key] = map[string]interface{}{"type": inputType.(string)}
							workflowInputs[key] = inputType.(string)
						}
						if _, ok := newInputs["pippy_run_id"]; !ok {
							newInputs["pippy_run_id"] = map[string]string{"type": "string"}

							change, err := yaml.Marshal(map[string]interface{}{"workflow_dispatch": map[string]interface{}{"inputs": newInputs}})
							if err != nil {
								return nil, nil, err
							}
							requiredChanges = append(requiredChanges, string(change))
						}
					}
				} else {
					change, err := yaml.Marshal(map[string]interface{}{"workflow_dispatch": map[string]interface{}{"inputs": map[string]interface{}{"pippy_run_id": map[string]interface{}{"type": "string"}}}})
					if err != nil {
						return nil, nil, err
					}
					requiredChanges = append(requiredChanges, string(change))
				}
			} else {
				newDispatch := map[string]interface{}{"workflow_dispatch": map[string]interface{}{"inputs": map[string]interface{}{"pippy_run_id": map[string]interface{}{"type": "string"}}}}
				changes, err := yaml.Marshal(map[string]interface{}{"on": newDispatch})
				if err != nil {
					return nil, nil, err
				}
				requiredChanges = append(requiredChanges, string(changes))
			}
		} else {
			// this is an array
			dispatches := onDispatches.([]interface{})
			newDispatch := make(map[string]interface{})
			for _, dispatch := range dispatches {
				newDispatch[dispatch.(string)] = map[string]interface{}{}
			}
			newDispatch["workflow_dispatch"] = map[string]interface{}{"inputs": map[string]interface{}{"pippy_run_id": map[string]interface{}{"type": "string"}}}
			change, err := yaml.Marshal(map[string]interface{}{"on": newDispatch})
			if err != nil {
				return nil, nil, err
			}
			requiredChanges = append(requiredChanges, string(change))
		}
	} else {
		newDispatch := map[string]interface{}{"workflow_dispatch": map[string]interface{}{"inputs": map[string]interface{}{"pippy_run_id": map[string]interface{}{"type": "string"}}}}
		changes, err := yaml.Marshal(map[string]interface{}{"on": newDispatch})
		if err != nil {
			return nil, nil, err
		}
		requiredChanges = append(requiredChanges, string(changes))
	}

	if !strings.Contains(workflowRunName, "inputs.pippy_run_id") {
		if workflowRunName == "" {
			workflowRunName = workflowName
		}
		change, err := yaml.Marshal(map[string]interface{}{"run-name": fmt.Sprintf("%s - ${{inputs.pippy_run_id}}", workflowRunName)})
		if err != nil {
			return nil, nil, err
		}
		requiredChanges = append(requiredChanges, string(change))
	}

	return requiredChanges, workflowInputs, nil
}

func (g *Github) ValidateWorkflowFull(org, repo, path string) (string, string, error) {
	client, err := g.New()
	if err != nil {
		return "", "", err
	}

	opt := &github.RepositoryContentGetOptions{}
	fileContent, _, _, err := client.Repositories.GetContents(context.Background(), org, repo, path, opt)
	if err != nil {
		return "", "", err
	}

	oldFile, err := fileContent.GetContent()
	if err != nil {
		return "", "", err
	}

	workflowsDef := make(map[string]interface{})
	err = yaml.Unmarshal([]byte(oldFile), workflowsDef)
	if err != nil {
		return oldFile, "", err
	}

	var workflowName string
	var workflowRunName string
	//workflowDispatchInputs := make(map[string]string)

	if value, ok := workflowsDef["name"]; ok {
		workflowName = value.(string)
	}

	if value, ok := workflowsDef["run-name"]; ok {
		workflowRunName = value.(string)
	}

	workflowInputs := make(map[string]string)
	if onDispatches, ok := workflowsDef["on"]; ok {
		if dispatches, ok := onDispatches.(map[string]interface{}); ok {
			if workflowDispatch, ok := dispatches["workflow_dispatch"]; ok {
				if workflowDispatchInputs, ok := workflowDispatch.(map[string]interface{}); ok {
					if inputs, ok := workflowDispatchInputs["inputs"]; ok {
						newInputs := make(map[string]interface{})
						for key, value := range inputs.(map[string]interface{}) {
							inputType := value.(map[string]interface{})["type"]
							newInputs[key] = map[string]interface{}{"type": inputType.(string)}
							workflowInputs[key] = inputType.(string)
						}
						if _, ok := newInputs["pippy_run_id"]; !ok {
							newInputs["pippy_run_id"] = map[string]string{"type": "string"}
							workflowDispatchInputs["inputs"] = map[string]interface{}{"inputs": newInputs}
						}
					}
				} else {
					dispatches["workflow_dispatch"] = map[string]interface{}{"inputs": map[string]interface{}{"pippy_run_id": map[string]interface{}{"type": "string"}}}
				}
			} else {
				dispatches["workflow_dispatch"] = map[string]interface{}{"inputs": map[string]interface{}{"pippy_run_id": map[string]interface{}{"type": "string"}}}
			}
		} else {
			// this is an array
			dispatches := onDispatches.([]interface{})
			newDispatch := make(map[string]interface{})
			for _, dispatch := range dispatches {
				newDispatch[dispatch.(string)] = map[string]interface{}{}
			}
			newDispatch["workflow_dispatch"] = map[string]interface{}{"inputs": map[string]interface{}{"pippy_run_id": map[string]interface{}{"type": "string"}}}
			workflowsDef["on"] = newDispatch
		}
	} else {
		newDispatch := map[string]interface{}{"workflow_dispatch": map[string]interface{}{"inputs": map[string]interface{}{"pippy_run_id": map[string]interface{}{"type": "string"}}}}
		workflowsDef["on"] = newDispatch
	}

	if !strings.Contains(workflowRunName, "inputs.pippy_run_id") {
		if workflowRunName == "" {
			workflowRunName = workflowName
		}
		workflowsDef["run-name"] = fmt.Sprintf("%s - ${{inputs.pippy_run_id}}", workflowRunName)
	}

	newFile, err := yaml.Marshal(workflowsDef)
	if err != nil {
		return oldFile, "", err
	}

	return oldFile, string(newFile), nil
}
