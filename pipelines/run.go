package pipelines

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/nixmade/pippy/github"
	"github.com/nixmade/pippy/helpers"
	"github.com/nixmade/pippy/log"
	"github.com/nixmade/pippy/store"
	"github.com/nixmade/pippy/users"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/google/uuid"
	"github.com/nixmade/orchestrator/core"
	"github.com/rs/zerolog"
)

type State string

const (
	APP_NAME               = "pippy"
	SUCCESS          State = "Success"
	FAILED           State = "Failed"
	IN_PROGRESS      State = "InProgress"
	PENDING_APPROVAL State = "PendingApproval"
	PAUSED           State = "Paused"
	ROLLBACK         State = "Rollback"
	CONCURRENT_ERROR State = "ConcurrentError"
	CANCELED         State = "Canceled"
	LOCKED           State = "Locked"
)

type StageRunApproval struct {
	Name  string `json:"name,omitempty"`
	Login string `json:"login,omitempty"`
	Email string `json:"email,omitempty"`
}

type StageRunMetadata struct {
	Approval StageRunApproval `json:"approval,omitempty"`
}

type TriggerMetadata struct {
	Name   string `json:"name"`
	Login  string `json:"login"`
	Email  string `json:"email"`
	Reason string `json:"reason"`
}

type StageRun struct {
	Name            string            `json:"name"`
	State           string            `json:"state"`
	Url             string            `json:"url"`
	RunId           string            `json:"run_id"`
	Started         time.Time         `json:"started"`
	Completed       time.Time         `json:"completed"`
	Title           string            `json:"title"`
	Reason          string            `json:"reason"`
	Input           map[string]string `json:"input"`
	Rollback        *StageRun         `json:"rollback,omitempty"`
	Metadata        StageRunMetadata  `json:"metadata,omitempty"`
	ConcurrentRunId string            `json:"concurrent"`
}

type PipelineRun struct {
	Id           string            `json:"id"`
	PipelineName string            `json:"name"`
	Stages       []StageRun        `json:"stages"`
	State        string            `json:"state"`
	Created      time.Time         `json:"created"`
	Updated      time.Time         `json:"updated"`
	Inputs       map[string]string `json:"input"`
	Paused       bool              `json:"paused"`
	Version      string            `json:"version"`
	Trigger      TriggerMetadata   `json:"trigger_metadata"`
}

type run struct {
	state           string
	runUrl          string
	runId           string
	title           string
	started         time.Time
	completed       time.Time
	rollback        *run
	reason          string
	approvedBy      string
	version         string
	inputs          map[string]string
	concurrentRunId string
}

type status struct {
	m     map[string]*run
	state State
	lock  sync.RWMutex
	cache map[string]*run
}

func (s *status) Set(key string, value *run) {
	s.lock.Lock()
	defer s.lock.Unlock()
	s.m[key] = value
	s.cache = make(map[string]*run, len(s.m))
	for k, v := range s.m {
		s.cache[k] = deepCopy(&run{}, v)
	}
}

func deepCopy(to *run, from *run) *run {
	*to = *from
	to.inputs = make(map[string]string, len(from.inputs))
	for k, v := range from.inputs {
		to.inputs[k] = v
	}
	if from.rollback != nil {
		to.rollback = deepCopy(&run{}, from.rollback)
	}
	return to
}

func (s *status) Get(key string) *run {
	s.lock.RLock()
	defer s.lock.RUnlock()
	value, ok := s.m[key]
	if !ok {
		return &run{}
	}

	return value
}

func (s *status) GetCache(key string) *run {
	s.lock.RLock()
	defer s.lock.RUnlock()

	value, ok := s.cache[key]
	if !ok {
		return &run{}
	}

	return value
}

func (s *status) UpdateState(state State) {
	s.lock.Lock()
	defer s.lock.Unlock()
	s.state = state
}

func (s *status) GetState() State {
	s.lock.Lock()
	defer s.lock.Unlock()
	return s.state
}

func getStageName(i int, name string) string {
	return fmt.Sprintf("%s-%d", name, i)
}

func getTargetName(i int, stage Stage) string {
	return fmt.Sprintf("%s/%d/%d", stage.Repo, i, stage.Workflow.Id)
}

func createOrchestrator(ctx context.Context, name, runId string, inputs map[string]string, templateValues map[string]string, trigger TriggerMetadata, force bool) (*orchestrator, error) {
	pipeline, err := GetPipeline(ctx, name)
	if err != nil {
		return nil, err
	}

	if runId == "" {
		runId = uuid.New().String()
	}

	logger := log.Get().With().Str("Pipeline", name).Str("RunId", runId).Logger()

	for _, stage := range pipeline.Stages {
		for key, value := range stage.Input {
			if templateValue, ok := templateValues[value]; ok {
				if _, ok := inputs[key]; ok {
					logger.Info().Str("TemplateValue", value).Msg("Skip replacing template value since input is defined")
					continue
				}
				inputs[key] = templateValue
			}
		}
	}

	stageStatus := &status{m: make(map[string]*run)}
	o := &orchestrator{
		pipeline:      pipeline,
		stageStatus:   stageStatus,
		pipelineRunId: runId,
		inputs:        inputs,
		done:          make(chan bool, 1),
		logger:        &logger,
		paused:        false,
		githubClient:  &github.Github{Context: ctx},
		targetVersion: runId,
		started:       time.Now().UTC(),
		force:         force,
		trigger:       trigger,
	}

	if err = o.setConfig(ctx); err != nil {
		return nil, err
	}

	if err := o.loadPipelineRun(ctx); err != nil {
		return nil, err
	}

	return o, nil
}

func RunPipeline(ctx context.Context, name, runId string, inputs map[string]string, templateValues map[string]string, trigger TriggerMetadata, force bool) error {
	o, err := createOrchestrator(ctx, name, runId, inputs, templateValues, trigger, force)
	if err != nil {
		return err
	}

	currentState := o.stageStatus.GetState()
	if currentState == SUCCESS || currentState == FAILED {
		o.logger.Warn().Str("State", string(currentState)).Msg("Rollout already completed")
		return nil
	}

	if err := o.orchestrate(ctx, 5000); err != nil {
		o.logger.Error().Err(err).Msg("Failed to run async orchestrator")
		//panic(err)
		return err
	}
	o.logger.Info().Msg("orchestrator is done")

	return nil
}

func RunPipelineUI(name, runId string, inputs map[string]string, force bool) error {
	userStore, err := users.GetCachedTokens()
	if err != nil {
		return err
	}

	trigger := TriggerMetadata{Name: userStore.GithubUser.Name, Login: userStore.GithubUser.Login, Email: userStore.GithubUser.Email, Reason: "Manual run"}
	o, err := createOrchestrator(context.Background(), name, runId, inputs, nil, trigger, force)
	if err != nil {
		return err
	}

	o.run(context.Background())
	defer o.wait()

	// return nil
	p := tea.NewProgram(initialModel(o.pipeline, o.stageStatus, o.started.String(), o.pipelineRunId))
	if _, err := p.Run(); err != nil {
		o.logger.Error().Err(err).Msg("error running UI")
		return err
	}
	return nil
}

type rollbackInfo struct {
	inputs map[string]string
}

type orchestrator struct {
	engine        *core.Engine
	options       *core.RolloutOptions
	pipeline      *Pipeline
	started       time.Time
	stageStatus   *status
	pipelineRunId string
	inputs        map[string]string
	logger        *zerolog.Logger
	done          chan bool
	paused        bool
	wg            sync.WaitGroup
	config        *core.Config
	githubClient  github.Client
	rollback      *rollbackInfo
	targetVersion string
	force         bool
	trigger       TriggerMetadata
}

func (o *orchestrator) setConfig(ctx context.Context) error {
	homedir, err := store.GetHomeDir()
	if err != nil {
		o.logger.Error().Err(err).Msg("failed to get home directory")
		return err
	}

	config := core.NewDefaultConfig()

	config.ApplicationName = APP_NAME
	if os.Getenv("DATABASE_URL") != "" {
		config.StoreDatabaseURL = os.Getenv("DATABASE_URL")
		config.StoreDatabaseSchema = store.PUBLIC_SCHEMA
		config.StoreDatabaseTable = "orchestrator"
		schema := ctx.Value(store.DatabaseSchemaCtx)
		if schema != nil {
			config.StoreDatabaseSchema = schema.(string)
		}
	} else {
		config.StoreDirectory = path.Join(homedir, ".pippy", "db", "orchestrator")
	}
	config.LogDirectory = path.Join(homedir, ".pippy", "logs")
	config.LogLevel = "debug"
	config.ConsoleLogging = false
	o.config = config

	return nil
}

type MonitoringController struct {
	*DatadogInfo
}

type MonitorState struct {
	Name                 string `json:"name"`
	OverallState         string `json:"overall_state"`
	OverallStateModified string `json:"overall_state_modified"`
}

func (m *MonitoringController) ExternalMonitoring(_ []*core.ClientState) error {
	for _, monitor := range m.Monitors {
		monitorID, err := strconv.ParseInt(monitor, 10, 64)
		if err != nil {
			return fmt.Errorf("failed to parse monitor %s", monitor)
		}

		url := fmt.Sprintf("https://api.%s/api/v1/monitor/%d?group_states=all&with_downtimes=true", m.Site, monitorID)
		headers := map[string]string{
			"Accept":             "application/json",
			"DD-API-KEY":         m.ApiKey,
			"DD-APPLICATION-KEY": m.ApplicationKey,
		}
		response, err := helpers.HttpGet(url, headers)
		if err != nil {
			return fmt.Errorf("received error from datadog %s", err)
		}

		state := MonitorState{}
		if err := json.Unmarshal([]byte(response), &state); err != nil {
			return fmt.Errorf("failed to unmarshal datadog response %s", err)
		}

		if strings.EqualFold(state.OverallState, "alert") {
			return fmt.Errorf("monitor %s in alert state since %s", monitor, state.OverallStateModified)
		}
	}

	return nil
}

func (o *orchestrator) setupEngine() error {
	var err error
	o.logger.Info().Msg("setting up new orchestrator engine")

	o.engine, err = core.NewOrchestratorEngine(o.config)
	if err != nil {
		return err
	}

	core.RegisteredMonitoringControllers = append(core.RegisteredMonitoringControllers, &MonitoringController{})

	o.options = &core.RolloutOptions{
		BatchPercent:        1,
		SuccessPercent:      100,
		SuccessTimeoutSecs:  0,
		DurationTimeoutSecs: 3600,
	}

	if err = o.engine.SetRolloutOptions(APP_NAME, o.pipeline.Name, o.options); err != nil {
		o.logger.Error().Err(err).EmbedObject(o.options).Msg("failed to set rollout options")
		return err
	}

	if err = o.engine.SetTargetVersion(APP_NAME, o.pipeline.Name, core.EntityTargetVersion{Version: o.pipelineRunId}); err != nil {
		o.logger.Error().Err(err).Msg("failed to set target version")
		return err
	}

	o.logger.Info().Msg("new orchestrator engine setup done")

	return nil
}

func (o *orchestrator) loadPipelineRun(ctx context.Context) error {
	pipelineRun, err := GetPipelineRun(ctx, o.pipeline.Name, o.pipelineRunId)
	if err != nil {
		if errors.Is(err, store.ErrKeyNotFound) {
			o.logger.Info().Msg("pipeline run not found, new run")
			return nil
		}
		o.logger.Error().Err(err).Msg("failed to get pipeline run")
		return err
	}

	o.stageStatus.UpdateState(State(pipelineRun.State))
	o.inputs = pipelineRun.Inputs
	o.started = pipelineRun.Created
	if pipelineRun.State == string(ROLLBACK) {
		o.rollback = &rollbackInfo{}
		o.targetVersion = pipelineRun.Version
	}

	for i, stageRun := range pipelineRun.Stages {
		o.stageStatus.Set(getStageName(i, stageRun.Name), loadStageRun(&stageRun))
	}

	return nil
}

func setStageRun(stageRun *StageRun, status *run) {
	if status == nil {
		return
	}
	stageRun.Input = make(map[string]string)
	stageRun.State = status.state
	stageRun.Url = status.runUrl
	stageRun.RunId = status.runId
	stageRun.Title = status.title
	stageRun.Started = status.started
	stageRun.Completed = status.completed
	stageRun.Reason = status.reason
	stageRun.ConcurrentRunId = status.concurrentRunId
	for key, value := range status.inputs {
		stageRun.Input[key] = value
	}
	if status.rollback != nil {
		if stageRun.Rollback == nil {
			stageRun.Rollback = &StageRun{Name: stageRun.Name}
		}
		setStageRun(stageRun.Rollback, status.rollback)
	}
}

func loadStageRun(stageRun *StageRun) *run {
	value := &run{
		state:           stageRun.State,
		runUrl:          stageRun.Url,
		runId:           stageRun.RunId,
		title:           stageRun.Title,
		started:         stageRun.Started,
		completed:       stageRun.Completed,
		reason:          stageRun.Reason,
		concurrentRunId: stageRun.ConcurrentRunId,
	}
	approval := stageRun.Metadata.Approval
	if approval.Name != "" || approval.Login != "" {
		value.approvedBy = fmt.Sprintf("%s(%s)", approval.Name, approval.Login)
	}
	if stageRun.Rollback != nil {
		value.rollback = loadStageRun(stageRun.Rollback)
	}
	return value
}

func (o *orchestrator) savePipelineRun(ctx context.Context) error {
	o.logger.Info().Msg("begin saving pipeline run")
	pipelineRun, err := GetPipelineRun(ctx, o.pipeline.Name, o.pipelineRunId)
	if errors.Is(err, store.ErrKeyNotFound) {
		o.logger.Warn().Msg("pipeline run not found creating new")
		pipelineRun = &PipelineRun{Id: o.pipelineRunId, PipelineName: o.pipeline.Name, Paused: false, Created: time.Now().UTC(), Trigger: o.trigger}
	} else if err != nil {
		return nil
	}

	pipelineRun.State = string(o.stageStatus.GetState())
	pipelineRun.Updated = time.Now().UTC()
	pipelineRun.Inputs = o.inputs
	pipelineRun.Version = o.targetVersion
	o.paused = pipelineRun.Paused
	o.started = pipelineRun.Created

	var stages []StageRun
	for i, stage := range o.pipeline.Stages {
		stageName := getStageName(i, stage.Workflow.Name)
		stageRun := StageRun{Name: stage.Workflow.Name}
		for j, savedStageRun := range pipelineRun.Stages {
			if savedStageRun.Name == stage.Workflow.Name && i == j {
				stageRun = savedStageRun
			}
		}
		setStageRun(&stageRun, o.stageStatus.Get(stageName))
		stages = append(stages, stageRun)
	}
	pipelineRun.Stages = stages

	o.logger.Info().Msg("saving pipeline run")

	return savePipelineRun(ctx, pipelineRun)
}
