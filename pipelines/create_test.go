package pipelines

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"

	"github.com/nixmade/pippy/github"
	"github.com/nixmade/pippy/store"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	publicRepos = []github.Repo{
		{Name: "PublicDummy", Url: "DummyUrl", Detail: "This is it"},
	}
	privateRepos = []github.Repo{
		{Name: "PrivateDummy", Url: "DummyUrl", Detail: "This is it"},
	}

	expectedWorkflows = map[string][]github.Workflow{
		"org1/repo1": {
			{Name: "Workflow1", Id: 1234},
		},
	}

	ErrWorkflowsNotFound = errors.New("workflows not found for org/repo")
	ErrUnknownRepoType   = errors.New("unknown repo type specified")
)

type createGithubClient struct {
}

func (t *createGithubClient) ListRepos(repoType string) ([]github.Repo, error) {
	if repoType == "all" {
		return append(privateRepos, publicRepos...), nil
	}
	if repoType == "private" {
		return privateRepos, nil
	}
	if repoType == "public" {
		return publicRepos, nil
	}
	return nil, ErrUnknownRepoType
}
func (t *createGithubClient) ListWorkflows(org, repo string) ([]github.Workflow, error) {
	if workflows, ok := expectedWorkflows[fmt.Sprintf("%s/%s", org, repo)]; ok {
		return workflows, nil
	}
	return nil, ErrWorkflowsNotFound
}
func (t *createGithubClient) ListWorkflowRuns(org, repo string, workflowID int64, created string) ([]github.WorkflowRun, error) {
	return nil, nil
}
func (t *createGithubClient) CreateWorkflowDispatch(org, repo string, workflowID int64, ref string, inputs map[string]interface{}) error {
	return nil
}

func (t *createGithubClient) ValidateWorkflow(org, repo, path string) ([]string, map[string]string, error) {
	return nil, nil, nil
}

func (t *createGithubClient) ValidateWorkflowFull(org, repo, path string) (string, string, error) {
	return "", "", nil
}

func (t *createGithubClient) GetWorkflow(org, repo string, id int64) (*github.Workflow, error) {
	return nil, nil
}

func (t *createGithubClient) ListOrgsForUser() ([]github.Org, error) {
	return nil, nil
}

func TestGetRepos(t *testing.T) {
	github.DefaultClient = &createGithubClient{}

	repos, err := GetRepos("all")
	require.NoError(t, err)

	assert.ElementsMatch(t, []string{"PrivateDummy", "PublicDummy"}, repos, "Actual all repos does not match expected")

	repos, err = GetRepos("public")
	require.NoError(t, err)

	assert.ElementsMatch(t, []string{"PublicDummy"}, repos, "Actual public repos does not match expected")

	repos, err = GetRepos("private")
	require.NoError(t, err)

	assert.ElementsMatch(t, []string{"PrivateDummy"}, repos, "Actual private repos does not match expected")

	_, err = GetRepos("unknown")
	assert.ErrorIs(t, err, ErrUnknownRepoType)
}

func TestGetWorkflows(t *testing.T) {
	github.DefaultClient = &createGithubClient{}

	workflows, err := GetWorkflows("org1/repo1")

	require.NoError(t, err)

	assert.ElementsMatch(t, expectedWorkflows["org1/repo1"], workflows, "Actual workflows does not match expected")

	assert.ElementsMatch(t, []string{"Workflow1"}, getWorkflowTitles(workflows), "Actual workflows titles does not match expected")

	_, err = GetWorkflows("org1/repo2")

	assert.ErrorIs(t, err, ErrWorkflowsNotFound)
}

func TestSavePipeline(t *testing.T) {
	tempDir, err := os.MkdirTemp(os.TempDir(), "TestSavePipeline*")
	require.NoError(t, err)

	defer func() {
		assert.NoError(t, os.RemoveAll(tempDir))
	}()
	store.HomeDir = tempDir

	pipeline := &Pipeline{
		Name: "Pipeline1",
		Stages: []Stage{
			{Repo: "org1/repo1",
				Workflow: expectedWorkflows["org1/repo1"][0],
				Approval: false,
				Input:    map[string]string{"version": "dummy2"}},
		},
	}

	require.NoError(t, SavePipeline(context.Background(), pipeline))

	savedPipeline, err := GetPipeline(context.Background(), pipeline.Name)
	require.NoError(t, err)

	assert.Exactlyf(t, pipeline, savedPipeline, "Expected pipeline to match saved Pipeline")

	//os.RemoveAll(helper.homeDir)
}
