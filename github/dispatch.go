package github

import (
	"context"
	"fmt"

	"github.com/google/go-github/v66/github"
)

func (g *Github) CreateWorkflowDispatch(org, repo string, workflowID int64, ref string, inputs map[string]interface{}) error {
	client, err := g.New()
	if err != nil {
		return err
	}

	event := github.CreateWorkflowDispatchEventRequest{
		Ref:    ref,
		Inputs: inputs,
	}
	resp, err := client.Actions.CreateWorkflowDispatchEventByID(context.Background(), org, repo, workflowID, event)
	if err != nil {
		return err
	}

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}

	return fmt.Errorf("workflow create run dispatch returned error %s", resp.Status)
}
