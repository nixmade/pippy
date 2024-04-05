package pipelines

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/nixmade/orchestrator/core"
)

var (
	ErrReachedTerminalState = errors.New("pipeline rollout reached terminal state")
	ErrStageInProgress      = errors.New("pipeline stage still in progress")
)

func (o *orchestrator) orchestrate(ctx context.Context, interval int) error {
	if err := o.setupEngine(); err != nil {
		return err
	}
	defer o.engine.ShutdownAndClose()

	if err := o.tick(ctx, interval); err != nil {
		return err
	}

	return o.savePipelineRun(ctx)
}

func (o *orchestrator) tick(ctx context.Context, interval int) error {
	ticker := time.NewTicker(time.Duration(interval) * time.Millisecond)
	for {
		select {
		case <-o.done:
			o.logger.Info().Msg("orchestrator tick complete")
			ticker.Stop()
			return nil
		case <-ticker.C:
			o.logger.Info().Msg("orchestrator tick")
			if err := o.savePipelineRun(ctx); err != nil {
				return err
			}

			if o.paused {
				o.stageStatus.UpdateState(PAUSED)
				o.logger.Info().Msg("pipeline run paused")
				return nil
			}

			if o.targetVersion == "" {
				o.targetVersion = o.pipelineRunId
			}

			for i, stage := range o.pipeline.Stages {
				if err := o.stageTick(ctx, i, stage); err != nil {
					if errors.Is(err, ErrReachedTerminalState) {
						return nil
					}
					if errors.Is(err, ErrStageInProgress) {
						break
					}
					return err
				}
			}

			lastStageNum := len(o.pipeline.Stages) - 1
			currentRun := o.stageStatus.Get(getStageName(lastStageNum, o.pipeline.Stages[lastStageNum].Workflow.Name))
			if currentRun.state == "Success" {
				o.logger.Info().Msg("Rollout completely successfully")
				o.stageStatus.UpdateState(SUCCESS)
				return nil
			}
		}
	}
}

func (o *orchestrator) stageTick(ctx context.Context, i int, stage Stage) error {
	stageName := getStageName(i, stage.Workflow.Name)
	logger := o.logger.With().Str("Stage", stageName).Logger()
	currentRun := o.stageStatus.Get(stageName)

	if currentRun.state == "Success" || currentRun.state == "Failed" {
		logger.Info().Str("State", currentRun.state).Msg("rollout already completed for stage")
		return nil
	}

	if stage.Approval && currentRun.approvedBy == "" {
		logger.Info().Msg("Stage pending approval")
		currentRun.state = "PendingApproval"
		o.stageStatus.Set(stageName, currentRun)
		o.stageStatus.UpdateState(PENDING_APPROVAL)
		return ErrReachedTerminalState
	}

	// get the current state from github for above version
	if err := o.getCurrentState(i, stage); err != nil {
		return err
	}

	target, err := o.getStageTarget(ctx, i, stage)
	if err != nil {
		return err
	}

	stageCurrentRun := o.stageStatus.Get(stageName)
	currentRun = stageCurrentRun
	if o.rollback != nil {
		currentRun = currentRun.rollback
	}
	targetName := getTargetName(i, stage)
	logger.Info().Str("Target", targetName).Str("State", currentRun.state).Msg("orchestrating target")
	targets, err := o.engine.Orchestrate(APP_NAME, targetName, []*core.ClientState{target})
	if err != nil {
		logger.Error().Err(err).Msg("failed to orchestrate")
		return err
	}

	if o.rollback != nil {
		if currentRun.state == "Workflow_Success" {
			// We dont really care much about monitoring since we are in fast rollback mode
			currentRun.state = "Success"
			o.stageStatus.Set(stageName, stageCurrentRun)
			return nil
		}
		if currentRun.state == "Workflow_Failed" {
			currentRun.state = "Failed"
			o.stageStatus.Set(stageName, stageCurrentRun)
			return nil
		}
		return o.rolloutExpectedState(i, stage, targets)
	}

	rolloutState, err := o.engine.GetRolloutInfo(APP_NAME, targetName)
	if err != nil {
		logger.Error().Err(err).Msg("failed to get rollout info")
		return err
	}

	logger.Info().
		Str("LastKnownGoodVersion", rolloutState.LastKnownGoodVersion).
		Str("LastKnownBadVersion", rolloutState.LastKnownBadVersion).
		Str("TargetVersion", rolloutState.TargetVersion).
		Str("RollingVersion", rolloutState.RollingVersion).
		Msg("current rollout state")

	if strings.EqualFold(rolloutState.LastKnownGoodVersion, o.targetVersion) {
		currentRun.completed = time.Now().UTC()
		currentRun.state = "Success"
		o.stageStatus.Set(stageName, currentRun)
		logger.Info().Str("LastKnownGoodVersion", rolloutState.LastKnownGoodVersion).Msg("rollout completed successfully")
		return nil
	}

	if strings.EqualFold(rolloutState.LastKnownBadVersion, o.targetVersion) {
		if shouldRollback(stage, currentRun) && rolloutState.LastKnownGoodVersion != "" {
			// Rollback at this point
			logger.Info().Str("LastKnownBadVersion", rolloutState.LastKnownBadVersion).Msg("rolling back due to workflow or monitoring failure")
			o.stageRollback(ctx, i, stage, rolloutState.LastKnownGoodVersion)
			currentRun = o.stageStatus.Get(stageName)
		}
		currentRun.completed = time.Now().UTC()
		currentRun.state = "Failed"
		o.stageStatus.Set(stageName, currentRun)
		if currentRun.rollback != nil {
			o.stageStatus.UpdateState(ROLLBACK)
		} else {
			o.stageStatus.UpdateState(FAILED)
		}
		logger.Info().Str("LastKnownBadVersion", rolloutState.LastKnownBadVersion).Msg("rollout failed")
		return ErrReachedTerminalState
	}

	if !strings.EqualFold(rolloutState.RollingVersion, o.targetVersion) {
		currentRun.state = "ConcurrentError"
		currentRun.concurrentRunId = rolloutState.RollingVersion
		currentRun.reason = "concurrent rollout ongoing, wait or force version"
		o.stageStatus.Set(stageName, currentRun)
		o.stageStatus.UpdateState(FAILED)
		logger.Info().Str("TargetVersion", o.targetVersion).Str("RollingVersion", rolloutState.RollingVersion).Msg("concurrent rollout ongoing, wait or force version")
		return ErrReachedTerminalState
	}

	if err := o.rolloutExpectedState(i, stage, targets); err != nil {
		return err
	}

	return nil
}

func shouldRollback(stage Stage, currentRun *run) bool {
	if stage.Monitor.Workflow.Rollback && currentRun.state == "Workflow_Failed" {
		return true
	}

	if stage.Monitor.Datadog != nil && stage.Monitor.Datadog.Rollback {
		return true
	}

	return false
}

func (o *orchestrator) getCurrentState(i int, stage Stage) error {
	stageName := getStageName(i, stage.Workflow.Name)
	logger := o.logger.With().Str("Stage", stageName).Logger()
	logger.Info().Msg("getting current stage state")
	currentStageRun := o.stageStatus.Get(stageName)

	currentRun := currentStageRun
	if o.rollback != nil {
		currentRun = currentRun.rollback
	}

	// if we have already determined its a success or failure dont do anything
	// we might be in monitoring phase
	if currentRun.version == o.targetVersion && (currentRun.state == "Workflow_Success" || currentRun.state == "Workflow_Failed") {
		logger.Info().Str("CurrentRun", currentRun.version).Str("TargetVersion", o.targetVersion).Msg("using previously stored current run")
		return nil
	}

	stageRunId := ""
	if currentRun.runId == "" {
		stageRunId = uuid.New().String()
		currentRun.state = "Workflow_Unknown"
		currentRun.runId = stageRunId
		logger.Info().Str("RunId", stageRunId).Msg("creating new stage run id")
		o.stageStatus.Set(stageName, currentRun)
	} else {
		stageRunId = currentRun.runId
	}

	logger = logger.With().Str("RunId", stageRunId).Logger()

	orgRepoSlice := strings.SplitN(stage.Repo, "/", 2)
	logger.Info().Str("Org", orgRepoSlice[0]).Str("Repo", orgRepoSlice[1]).Int64("WorkflowId", stage.Workflow.Id).Msg("listing github workflows")
	created := fmt.Sprintf(">=%s", o.started)
	workflowRuns, err := o.githubClient.ListWorkflowRuns(orgRepoSlice[0], orgRepoSlice[1], stage.Workflow.Id, created)
	if err != nil {
		logger.Error().Err(err).Str("Org", orgRepoSlice[0]).Str("Repo", orgRepoSlice[1]).Int64("WorkflowId", stage.Workflow.Id).Msg("error listing github workflows")
		return err
	}

	//currentRun.state = "Workflow_Unknown"
	currentRun.runId = stageRunId
	currentRun.version = ""
	successStates := []string{"completed", "success", "skipped"}
	inProgressStates := []string{"in_progress", "queued", "requested", "waiting", "pending"}
	for _, workflowRun := range workflowRuns {
		if !strings.Contains(workflowRun.Name, stageRunId) {
			continue
		}
		currentRun.runUrl = workflowRun.Url
		currentRun.title = workflowRun.Name
		currentRun.version = o.targetVersion
		if slices.Contains(successStates, workflowRun.Status) {
			if workflowRun.Conclusion == "success" || workflowRun.Conclusion == "" || stage.Monitor.Workflow.Ignore {
				currentRun.completed = workflowRun.UpdatedAt
				currentRun.state = "Workflow_Success"
				logger.Info().Str("WorkflowRun", workflowRun.Name).Str("WorkflowRunUrl", workflowRun.Url).Msg("github workflow run completed successfully")
				break
			}
		}
		if slices.Contains(inProgressStates, workflowRun.Status) {
			currentRun.state = "InProgress"
			logger.Info().Str("WorkflowRun", workflowRun.Name).Str("WorkflowRunUrl", workflowRun.Url).Msg("github workflow run still in progress")
			break
		}
		currentRun.completed = workflowRun.UpdatedAt
		currentRun.reason = "github workflow run failed"
		currentRun.state = "Workflow_Failed"
		logger.Info().Str("WorkflowRun", workflowRun.Name).Str("WorkflowRunUrl", workflowRun.Url).Msg("github workflow run failed")
		break
	}
	o.stageStatus.Set(stageName, currentStageRun)

	return nil
}

func (o *orchestrator) getStageTarget(ctx context.Context, i int, stage Stage) (*core.ClientState, error) {
	stageName := getStageName(i, stage.Workflow.Name)
	logger := o.logger.With().Str("Stage", stageName).Logger()
	targetName := getTargetName(i, stage)
	isError := false
	currentRun := o.stageStatus.Get(stageName)
	if o.rollback != nil {
		currentRun = currentRun.rollback
		message := fmt.Sprintf("target state %s", currentRun.state)
		if currentRun.state == "Workflow_Failed" || currentRun.state == "InProgress" {
			isError = true
		}
		return &core.ClientState{Name: targetName, Version: currentRun.version, IsError: isError, Message: message}, nil
	}
	message := fmt.Sprintf("target state %s", currentRun.state)
	if currentRun.state == "Workflow_Failed" {
		isError = true
		o.options = &core.RolloutOptions{
			BatchPercent:        1,
			SuccessPercent:      100,
			SuccessTimeoutSecs:  0,
			DurationTimeoutSecs: 0,
		}

		logger.Info().EmbedObject(o.options).Msg("resetting rollout options")
		if err := o.engine.SetRolloutOptions(APP_NAME, targetName, o.options); err != nil {
			logger.Error().Err(err).EmbedObject(o.options).Msg("failed to set rollout options")
			return nil, err
		}
	} else if currentRun.state == "Workflow_Unknown" {
		o.options = &core.RolloutOptions{
			BatchPercent:        1,
			SuccessPercent:      100,
			SuccessTimeoutSecs:  0,
			DurationTimeoutSecs: 3600,
		}

		if stage.Monitor.Datadog != nil {
			// just choosing 15mins for now, later add to datadog info as configurable
			o.options.SuccessTimeoutSecs = 900
		}

		logger.Info().EmbedObject(o.options).Msg("setting rollout options")
		if err := o.engine.SetRolloutOptions(APP_NAME, targetName, o.options); err != nil {
			logger.Error().Err(err).EmbedObject(o.options).Msg("failed to set rollout options")
			return nil, err
		}

		logger.Info().Str("TargetVersion", o.pipelineRunId).Msg("setting target version")
		if o.force {
			if err := o.engine.ForceTargetVersion(APP_NAME, targetName, core.EntityTargetVersion{Version: o.pipelineRunId}); err != nil {
				o.logger.Error().Err(err).Msg("failed to force target version")
				return nil, err
			}
			rolloutState, err := o.engine.GetRolloutInfo(APP_NAME, targetName)
			if err != nil {
				logger.Error().Err(err).Msg("failed to get rollout info")
				return nil, err
			}
			if rolloutState.LastKnownBadVersion != "" {
				if err := CancelPipelineRun(ctx, o.pipeline.Name, rolloutState.LastKnownBadVersion); err != nil {
					logger.Error().Str("RunId", rolloutState.LastKnownBadVersion).Err(err).Msg("failed to cancel pipeline run")
					return nil, err
				}
			}
		} else {
			if err := o.engine.SetTargetVersion(APP_NAME, targetName, core.EntityTargetVersion{Version: o.pipelineRunId}); err != nil {
				o.logger.Error().Err(err).Msg("failed to set target version")
				return nil, err
			}
		}

		if stage.Monitor.Datadog != nil {
			controller := &MonitoringController{DatadogInfo: stage.Monitor.Datadog}
			if err := o.engine.SetEntityMonitoringController(APP_NAME, targetName, controller); err != nil {
				o.logger.Error().Err(err).Msg("failed to set monitoring controller")
				return nil, err
			}
		}
	} else if currentRun.state == "InProgress" {
		isError = true
	}

	return &core.ClientState{Name: targetName, Version: currentRun.version, IsError: isError, Message: message}, nil
}

func (o *orchestrator) rolloutExpectedState(i int, stage Stage, targets []*core.ClientState) error {
	stageName := getStageName(i, stage.Workflow.Name)
	currentRun := o.stageStatus.Get(stageName)
	if o.rollback != nil {
		if currentRun.rollback.state == string(IN_PROGRESS) {
			return ErrStageInProgress
		}
	} else if currentRun.state == "InProgress" {
		o.stageStatus.UpdateState(IN_PROGRESS)
		return ErrStageInProgress
	}

	o.logger.Info().Msg("rolling out expected state")

	orgRepoSlice := strings.SplitN(stage.Repo, "/", 2)
	targetName := getTargetName(i, stage)
	for _, target := range targets {
		if !strings.EqualFold(targetName, target.Name) {
			continue
		}

		stageName := getStageName(i, stage.Workflow.Name)
		currentRun := o.stageStatus.Get(stageName)
		stageCurrentRun := currentRun
		if o.rollback != nil {
			currentRun = currentRun.rollback
		}

		if strings.EqualFold(currentRun.version, target.Version) {
			//fmt.Println(target.Name, target.Version, "Already running required version")
			break
		}

		if !strings.EqualFold(target.Version, o.targetVersion) {
			break
		}

		dynamicInputs := o.inputs
		if o.rollback != nil {
			dynamicInputs = o.rollback.inputs
		}

		// default pippy run id
		inputs := map[string]interface{}{
			"pippy_run_id": currentRun.runId,
		}

		// static key value pair from each stage
		for key, value := range stage.Input {
			// static values are little smarter
			// if they are empty do not add them to the list
			// we can override with dynamic key values
			if value == "" {
				continue
			}
			inputs[key] = value
		}

		// dynamic key value pair provided as input
		for key, value := range dynamicInputs {
			// if we defined a static value, skip setting it here
			// can be accidental by user
			if _, ok := inputs[key]; ok {
				o.logger.Warn().Str("Stage", targetName).Str("Org", orgRepoSlice[0]).Str("Repo", orgRepoSlice[1]).Int64("WorkflowId", stage.Workflow.Id).Str("Key", key).Msg("setting dynamic value since its defined as static")
			}
			inputs[key] = value
		}

		currentRun.started = time.Now().UTC()
		o.logger.Info().Str("Stage", targetName).Str("Org", orgRepoSlice[0]).Str("Repo", orgRepoSlice[1]).Int64("WorkflowId", stage.Workflow.Id).Msg("create a new github workflow run")
		if err := o.githubClient.CreateWorkflowDispatch(orgRepoSlice[0], orgRepoSlice[1], stage.Workflow.Id, "main", inputs); err != nil {
			o.logger.Error().Err(err).Str("Stage", targetName).Str("Org", orgRepoSlice[0]).Str("Repo", orgRepoSlice[1]).Int64("WorkflowId", stage.Workflow.Id).Msg("failed to create a new github workflow run")
			return err
		}
		currentRun.state = "InProgress"
		currentRun.inputs = make(map[string]string)
		for key, value := range inputs {
			currentRun.inputs[key] = value.(string)
		}
		o.stageStatus.Set(stageName, stageCurrentRun)
		if o.rollback != nil {
			o.stageStatus.UpdateState(IN_PROGRESS)
		}
		return ErrStageInProgress
	}

	if currentRun.state != "Success" {
		return ErrStageInProgress
	}

	return nil
}

func (o *orchestrator) stageRollback(ctx context.Context, i int, stage Stage, version string) {
	o.rollback = &rollbackInfo{}
	o.targetVersion = version
	// get inputs used during pipeline run
	o.stageStatus.UpdateState(ROLLBACK)

	stageName := getStageName(i, stage.Workflow.Name)
	currentRun := o.stageStatus.Get(stageName)
	if currentRun.rollback == nil {
		currentRun.rollback = &run{}
		o.stageStatus.Set(stageName, currentRun)
	}

	pipelineRun, err := GetPipelineRun(ctx, o.pipeline.Name, version)
	if err != nil {
		// error during rollback, just mark rollback failure below
		o.logger.Error().Str("Version", version).Err(err).Msg("failed to get previous successful pipeline run")
		currentRun.rollback.state = string(FAILED)
		currentRun.rollback.reason = fmt.Sprintf("failed to get previous successful pipeline run %s", version)
		o.stageStatus.Set(stageName, currentRun)
		return
	}
	o.rollback.inputs = pipelineRun.Inputs

	for err := ErrStageInProgress; err == ErrStageInProgress; {
		err = o.stageTick(ctx, i, stage)
	}
}

func (o *orchestrator) run(ctx context.Context) {
	o.wg.Add(1)

	go func() {
		defer o.wg.Done()
		if err := o.orchestrate(ctx, 5000); err != nil {
			o.logger.Error().Err(err).Msg("Failed to run async orchestrator")
			panic(err)
		}
		o.logger.Info().Msg("orchestrator is done")
	}()
}

func (o *orchestrator) wait() {
	o.done <- true
	o.logger.Info().Msg("waiting for orchestrator to exit")
	o.wg.Wait()
}
