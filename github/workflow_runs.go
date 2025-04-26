package github

import (
	"context"
	"time"

	"github.com/google/go-github/v71/github"
)

type WorkflowRun struct {
	Name, Status, Url, Conclusion string
	Id, WorkflowID                int64
	RunStartedAt, UpdatedAt       time.Time
}

func (i WorkflowRun) Title() string       { return i.Name }
func (i WorkflowRun) Description() string { return i.Url }
func (i WorkflowRun) FilterValue() string { return i.Name }

func (g *Github) ListWorkflowRuns(org, repo string, workflowID int64, created string) ([]WorkflowRun, error) {
	client, err := g.New()
	if err != nil {
		return nil, err
	}

	opt := &github.ListWorkflowRunsOptions{
		Event:   "workflow_dispatch",
		Created: created,
	}
	workflowRuns, _, err := client.Actions.ListWorkflowRunsByID(context.Background(), org, repo, workflowID, opt)
	if err != nil {
		return nil, err
	}

	var workflowItems []WorkflowRun

	for i := 0; i < len(workflowRuns.WorkflowRuns); i++ {
		workflowItems = append(workflowItems, WorkflowRun{
			Name:         workflowRuns.WorkflowRuns[i].GetDisplayTitle(),
			Url:          workflowRuns.WorkflowRuns[i].GetHTMLURL(),
			Id:           workflowRuns.WorkflowRuns[i].GetID(),
			Status:       workflowRuns.WorkflowRuns[i].GetStatus(),
			WorkflowID:   workflowRuns.WorkflowRuns[i].GetWorkflowID(),
			Conclusion:   workflowRuns.WorkflowRuns[i].GetConclusion(),
			RunStartedAt: workflowRuns.WorkflowRuns[i].GetRunStartedAt().Time,
			UpdatedAt:    workflowRuns.WorkflowRuns[i].GetUpdatedAt().Time,
		})
	}

	return workflowItems, nil
}
