package pipelines

import (
	"context"
	"maps"
	"os"
	"path/filepath"
	"testing"

	"github.com/nixmade/pippy/github"
	"github.com/nixmade/pippy/log"
	"github.com/nixmade/pippy/store"

	"github.com/google/uuid"
	"github.com/nixmade/orchestrator/core"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	defaultTestPipeline = &Pipeline{
		Name: "Pipeline1",
		Stages: []Stage{
			{Repo: "org1/repo1",
				Workflow: expectedWorkflows["org1/repo1"][0],
				Approval: false,
				Input:    map[string]string{"version": ""}},
		},
	}
)

func setConfig(o *orchestrator) {
	config := core.NewDefaultConfig()

	config.ApplicationName = "pippy_test"
	if testing.Verbose() {
		config.LogLevel = "debug"
	} else {
		config.LogLevel = "fatal"
	}
	config.ConsoleLogging = true
	o.config = config
}

type dispatch struct {
	org, repo string
	id        int64
	inputs    map[string]interface{}
}

type runGithubClient struct {
	workflowRuns  []github.WorkflowRun
	dispatches    []dispatch
	dispatchErr   error
	afterDispatch bool
	stageStatus   *status
}

func newTestGithubClient() *runGithubClient {
	return &runGithubClient{
		dispatchErr:   nil,
		afterDispatch: false,
	}
}

func (t *runGithubClient) ListRepos(repoType string) ([]github.Repo, error) {
	return nil, nil
}
func (t *runGithubClient) ListWorkflows(org, repo string) ([]github.Workflow, error) {
	return nil, nil
}
func (t *runGithubClient) ListWorkflowRuns(org, repo string, workflowID int64, created string) ([]github.WorkflowRun, error) {
	dispatched := false
	for _, dispatch := range t.dispatches {
		dispatched = dispatch.id == workflowID
	}
	if !t.afterDispatch || dispatched {
		if t.stageStatus != nil {
			status := t.stageStatus.Get(getStageName(0, "Workflow1"))
			if status.rollback != nil {
				return []github.WorkflowRun{{Name: status.rollback.runId, Status: "completed", Conclusion: "success"}}, nil
			}
		}
		return t.workflowRuns, nil
	}
	return nil, nil
}
func (t *runGithubClient) CreateWorkflowDispatch(org, repo string, workflowID int64, ref string, inputs map[string]interface{}) error {
	t.dispatches = append(t.dispatches, dispatch{org: org, repo: repo, id: workflowID, inputs: maps.Clone(inputs)})
	return t.dispatchErr
}

func (t *runGithubClient) ValidateWorkflow(org, repo, path string) ([]string, map[string]string, error) {
	return nil, nil, nil
}

func (t *runGithubClient) ValidateWorkflowFull(org, repo, path string) (string, string, error) {
	return "", "", nil
}

func (t *runGithubClient) GetWorkflow(org, repo string, id int64) (*github.Workflow, error) {
	return nil, nil
}

func (t *runGithubClient) ListOrgsForUser() ([]github.Org, error) {
	return nil, nil
}

func setupOrchestrator(*testing.T) *orchestrator {
	logger := zerolog.New(os.Stderr).With().Caller().Timestamp().Logger().Output(zerolog.ConsoleWriter{Out: os.Stderr})
	if testing.Verbose() {
		logger = logger.Level(zerolog.DebugLevel)
	} else {
		logger = logger.Level(zerolog.FatalLevel)
	}
	log.DefaultLogger = &logger
	runId := uuid.New().String()

	stageStatus := &status{m: make(map[string]*run)}
	o := &orchestrator{
		pipeline:      defaultTestPipeline,
		stageStatus:   stageStatus,
		pipelineRunId: runId,
		inputs:        map[string]string{"version": "dummy2"},
		done:          make(chan bool, 1),
		logger:        &logger,
		paused:        false,
		githubClient:  newTestGithubClient(),
		targetVersion: runId,
	}
	setConfig(o)
	return o
}

func TestSaveLoadPipelineRun(t *testing.T) {
	o := setupOrchestrator(t)

	tempDir, err := os.MkdirTemp(os.TempDir(), "TestLoadPipelineRun*")
	require.NoError(t, err)

	defer os.RemoveAll(tempDir)
	store.HomeDir = tempDir

	require.NoError(t, o.loadPipelineRun(context.Background()))

	o.stageStatus.UpdateState(IN_PROGRESS)
	stageName := getStageName(0, "Workflow1")
	stageRunId := uuid.NewString()
	value := run{
		state:  string(IN_PROGRESS),
		runUrl: "dummyurl",
		runId:  stageRunId,
		title:  "test title this is cool!",
	}

	o.stageStatus.Set(stageName, &value)

	require.NoError(t, o.savePipelineRun(context.Background()))
	o.stageStatus.UpdateState(PENDING_APPROVAL)
	o.stageStatus.Set(stageName, &run{})

	require.NoError(t, o.loadPipelineRun(context.Background()))

	stageRun := o.stageStatus.Get(stageName)
	require.NotNil(t, stageRun)

	assert.Equal(t, "test title this is cool!", stageRun.title)
	assert.Equal(t, stageRunId, stageRun.runId)
}

func TestOrchestrateGood(t *testing.T) {
	o := setupOrchestrator(t)

	tempDir, err := os.MkdirTemp(os.TempDir(), "TestOrchestrateGood*")
	require.NoError(t, err)

	defer os.RemoveAll(tempDir)
	store.HomeDir = tempDir

	newPipeline := &Pipeline{
		Name: "Pipeline1",
		Stages: []Stage{
			{Repo: "org1/repo1",
				Workflow: expectedWorkflows["org1/repo1"][0],
				Approval: false,
				Input:    map[string]string{"version": ""}},
			{Repo: "org1/repo1",
				Workflow: github.Workflow{Name: "Workflow2", Id: 2345},
				Approval: false,
				Input:    map[string]string{"version": ""}},
		},
	}
	o.pipeline = newPipeline

	var runs []github.WorkflowRun
	for i, stage := range newPipeline.Stages {
		err = o.getCurrentState(i, stage)
		require.NoError(t, err)

		status := o.stageStatus.Get(getStageName(i, stage.Workflow.Name))
		require.NotNil(t, status)
		runs = append(runs, github.WorkflowRun{Name: status.runId, Status: "completed", Conclusion: ""})
	}

	githubClient := &runGithubClient{dispatchErr: nil, workflowRuns: runs, afterDispatch: true}
	o.githubClient = githubClient
	require.NoError(t, o.orchestrate(context.Background(), 1))

	require.Equal(t, SUCCESS, o.stageStatus.GetState())
	require.Len(t, githubClient.dispatches, 2)
}

func TestOrchestrateBad(t *testing.T) {
	o := setupOrchestrator(t)

	tempDir, err := os.MkdirTemp(os.TempDir(), "TestOrchestrateBad*")
	require.NoError(t, err)

	defer os.RemoveAll(tempDir)
	store.HomeDir = tempDir

	newPipeline := &Pipeline{
		Name: "Pipeline1",
		Stages: []Stage{
			{Repo: "org1/repo1",
				Workflow: expectedWorkflows["org1/repo1"][0],
				Approval: false,
				Input:    map[string]string{"version": ""}},
			{Repo: "org1/repo1",
				Workflow: github.Workflow{Name: "Workflow2", Id: 2345},
				Approval: false,
				Input:    map[string]string{"version": ""}},
		},
	}
	o.pipeline = newPipeline
	o.githubClient = newTestGithubClient()
	var runs []github.WorkflowRun
	for i, stage := range newPipeline.Stages {
		err = o.getCurrentState(i, stage)
		require.NoError(t, err)

		status := o.stageStatus.Get(getStageName(i, stage.Workflow.Name))
		require.NotNil(t, status)
		runs = append(runs, github.WorkflowRun{Name: status.runId, Status: "completed", Conclusion: "failure"})
	}

	githubClient := &runGithubClient{dispatchErr: nil, workflowRuns: runs, afterDispatch: true}
	o.githubClient = githubClient
	require.NoError(t, o.orchestrate(context.Background(), 1))

	require.Equal(t, FAILED, o.stageStatus.GetState())

	currentRun := o.stageStatus.Get(getStageName(1, newPipeline.Stages[1].Workflow.Name))
	require.NotNil(t, currentRun)
	require.Equal(t, "Workflow_Unknown", currentRun.state)
	require.Len(t, githubClient.dispatches, 1)
}

func TestOrchestrateIgnoreFailures(t *testing.T) {
	o := setupOrchestrator(t)

	tempDir, err := os.MkdirTemp(os.TempDir(), "TestOrchestrateBad*")
	require.NoError(t, err)

	defer os.RemoveAll(tempDir)
	store.HomeDir = tempDir

	newPipeline := &Pipeline{
		Name: "Pipeline1",
		Stages: []Stage{
			{Repo: "org1/repo1",
				Workflow: expectedWorkflows["org1/repo1"][0],
				Approval: false,
				Input:    map[string]string{"version": ""},
				Monitor:  MonitorInfo{Workflow: WorkflowInfo{Ignore: true}}},
			{Repo: "org1/repo1",
				Workflow: github.Workflow{Name: "Workflow2", Id: 2345},
				Approval: false,
				Input:    map[string]string{"version": ""},
				Monitor:  MonitorInfo{Workflow: WorkflowInfo{Ignore: true}}},
		},
	}
	o.pipeline = newPipeline
	o.githubClient = newTestGithubClient()
	var runs []github.WorkflowRun
	for i, stage := range newPipeline.Stages {
		err = o.getCurrentState(i, stage)
		require.NoError(t, err)

		status := o.stageStatus.Get(getStageName(i, stage.Workflow.Name))
		require.NotNil(t, status)
		runs = append(runs, github.WorkflowRun{Name: status.runId, Status: "completed", Conclusion: "failure"})
	}

	githubClient := &runGithubClient{dispatchErr: nil, workflowRuns: runs, afterDispatch: true}
	o.githubClient = githubClient
	require.NoError(t, o.orchestrate(context.Background(), 1))

	require.Equal(t, SUCCESS, o.stageStatus.GetState())

	for i, stage := range newPipeline.Stages {
		currentRun := o.stageStatus.Get(getStageName(i, stage.Workflow.Name))
		require.NotNil(t, currentRun)
		assert.Equal(t, "Success", currentRun.state)
	}
	require.Len(t, githubClient.dispatches, 2)
}

func TestOrchestrateApproval(t *testing.T) {
	o := setupOrchestrator(t)

	tempDir, err := os.MkdirTemp(os.TempDir(), "TestOrchestrateApproval*")
	require.NoError(t, err)

	defer os.RemoveAll(tempDir)
	store.HomeDir = tempDir

	newPipeline := &Pipeline{
		Name: "Pipeline1",
		Stages: []Stage{
			{Repo: "org1/repo1",
				Workflow: expectedWorkflows["org1/repo1"][0],
				Approval: false,
				Input:    map[string]string{"version": ""}},
			{Repo: "org1/repo1",
				Workflow: github.Workflow{Name: "Workflow2", Id: 2345},
				Approval: true,
				Input:    map[string]string{"version": ""}},
		},
	}
	o.pipeline = newPipeline
	setConfig(o)
	testGithubClient := newTestGithubClient()
	o.githubClient = testGithubClient
	err = o.getCurrentState(0, newPipeline.Stages[0])
	require.NoError(t, err)

	status := o.stageStatus.Get(getStageName(0, "Workflow1"))
	require.NotNil(t, status)
	runs := []github.WorkflowRun{
		{Name: status.runId, Status: "completed", Conclusion: ""},
	}
	testGithubClient = &runGithubClient{dispatchErr: nil, workflowRuns: runs, afterDispatch: true}
	o.githubClient = testGithubClient

	require.NoError(t, o.orchestrate(context.Background(), 1))

	require.Equal(t, PENDING_APPROVAL, o.stageStatus.GetState())
	require.Len(t, testGithubClient.dispatches, 1)
}

func TestOrchestratePaused(t *testing.T) {
	o := setupOrchestrator(t)

	tempDir, err := os.MkdirTemp(os.TempDir(), "TestOrchestratePaused*")
	require.NoError(t, err)

	defer os.RemoveAll(tempDir)
	store.HomeDir = tempDir

	o.githubClient = newTestGithubClient()

	require.NoError(t, o.savePipelineRun(context.Background()))

	pipelineRun, err := GetPipelineRun(context.Background(), defaultTestPipeline.Name, o.pipelineRunId)
	require.NoError(t, err)

	pipelineRun.Paused = true
	require.NoError(t, savePipelineRun(context.Background(), pipelineRun))

	require.NoError(t, o.orchestrate(context.Background(), 1))

	assert.Equal(t, PAUSED, o.stageStatus.GetState())
}

func TestOrchestrateApprovalMulti(t *testing.T) {
	o := setupOrchestrator(t)

	tempDir, err := os.MkdirTemp(os.TempDir(), "TestOrchestrateApproval*")
	require.NoError(t, err)

	defer os.RemoveAll(tempDir)
	store.HomeDir = tempDir

	newPipeline := &Pipeline{
		Name: "Pipeline1",
		Stages: []Stage{
			{Repo: "org1/repo1",
				Workflow: expectedWorkflows["org1/repo1"][0],
				Approval: false,
				Input:    map[string]string{"version": ""}},
			{Repo: "org1/repo1",
				Workflow: github.Workflow{Name: "Workflow2", Id: 2345},
				Approval: true,
				Input:    map[string]string{"version": ""}},
		},
	}
	o.pipeline = newPipeline
	{

		testGithubClient := newTestGithubClient()
		o.githubClient = testGithubClient
		err = o.getCurrentState(0, newPipeline.Stages[0])
		require.NoError(t, err)

		status := o.stageStatus.Get(getStageName(0, "Workflow1"))
		require.NotNil(t, status)
		runs := []github.WorkflowRun{
			{Name: status.runId, Status: "completed", Conclusion: ""},
		}
		testGithubClient = &runGithubClient{dispatchErr: nil, workflowRuns: runs, afterDispatch: true}
		o.githubClient = testGithubClient

		require.NoError(t, o.orchestrate(context.Background(), 1))

		require.Equal(t, PENDING_APPROVAL, o.stageStatus.GetState())
		require.Len(t, testGithubClient.dispatches, 1)
	}

	o2 := setupOrchestrator(t)
	o2.pipeline = newPipeline
	setConfig(o2)
	{
		testGithubClient := newTestGithubClient()
		o2.githubClient = testGithubClient
		err = o2.getCurrentState(0, newPipeline.Stages[0])
		require.NoError(t, err)

		status := o2.stageStatus.Get(getStageName(0, "Workflow1"))
		require.NotNil(t, status)
		runs := []github.WorkflowRun{
			{Name: status.runId, Status: "completed", Conclusion: "failure"},
		}
		testGithubClient = &runGithubClient{dispatchErr: nil, workflowRuns: runs, afterDispatch: true}
		o2.githubClient = testGithubClient

		require.NoError(t, o2.orchestrate(context.Background(), 1))

		status = o2.stageStatus.Get(getStageName(0, "Workflow1"))
		require.NotNil(t, status)
		assert.Equal(t, "Failed", status.state)
		assert.Equal(t, FAILED, o2.stageStatus.GetState())
		require.Len(t, testGithubClient.dispatches, 1)
	}

}

func TestOrchestrateRollback(t *testing.T) {
	o := setupOrchestrator(t)

	tempDir, err := os.MkdirTemp(os.TempDir(), "TestOrchestrateRollback*")
	require.NoError(t, err)

	defer os.RemoveAll(tempDir)
	store.HomeDir = tempDir

	o.config.StoreDirectory = filepath.Join(tempDir, "orchestrator")
	require.NoError(t, os.MkdirAll(o.config.StoreDirectory, os.ModePerm))

	newPipeline := &Pipeline{
		Name: "Pipeline1",
		Stages: []Stage{
			{Repo: "org1/repo1",
				Workflow: expectedWorkflows["org1/repo1"][0],
				Approval: false,
				Monitor:  MonitorInfo{Workflow: WorkflowInfo{Rollback: true, Ignore: false}},
				Input:    map[string]string{"version": ""}},
		},
	}
	o.pipeline = newPipeline

	err = o.getCurrentState(0, newPipeline.Stages[0])
	require.NoError(t, err)

	stageRun := o.stageStatus.Get(getStageName(0, "Workflow1"))
	require.NotNil(t, stageRun)

	runs := []github.WorkflowRun{
		{Name: stageRun.runId, Status: "completed", Conclusion: ""},
	}

	githubClient := &runGithubClient{dispatchErr: nil, workflowRuns: runs, afterDispatch: true}
	o.githubClient = githubClient
	require.NoError(t, o.orchestrate(context.Background(), 1))

	assert.Equal(t, SUCCESS, o.stageStatus.GetState())

	// LKG is established, lets try to rollout new
	prevRunId := o.pipelineRunId
	runId := uuid.New().String()
	o.pipelineRunId = runId
	o.targetVersion = runId
	o.stageStatus = &status{m: make(map[string]*run)}
	o.inputs = map[string]string{"version": "dummy4"}

	err = o.getCurrentState(0, defaultTestPipeline.Stages[0])
	require.NoError(t, err)

	stageRun = o.stageStatus.Get(getStageName(0, "Workflow1"))
	require.Equal(t, "Workflow_Unknown", stageRun.state)

	stageRun = o.stageStatus.Get(getStageName(0, "Workflow1"))
	require.NotNil(t, stageRun)

	runs = []github.WorkflowRun{
		{Name: stageRun.runId, Status: "completed", Conclusion: "failure"},
	}

	githubClient = &runGithubClient{dispatchErr: nil, workflowRuns: runs, afterDispatch: true, stageStatus: o.stageStatus}
	o.githubClient = githubClient

	// Need to get the right run id here for rollback
	// otherwise we enter an infinite loop
	require.NoError(t, o.orchestrate(context.Background(), 1))

	stageRun = o.stageStatus.Get(getStageName(0, "Workflow1"))
	require.NotNil(t, stageRun)
	require.NotNil(t, stageRun.rollback)

	require.Equal(t, string(FAILED), stageRun.state)
	require.Equal(t, runId, stageRun.version)
	require.Equal(t, prevRunId, stageRun.rollback.version)
	require.Equal(t, "dummy4", stageRun.inputs["version"])
	require.Equal(t, "dummy2", stageRun.rollback.inputs["version"])
	require.Len(t, githubClient.dispatches, 2)
}
