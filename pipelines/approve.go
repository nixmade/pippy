package pipelines

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/nixmade/pippy/audit"
	"github.com/nixmade/pippy/users"

	"github.com/charmbracelet/huh"
)

const (
	AUDIT_APPROVED        string = "Approved"
	AUDIT_CANCEL_APPROVAL string = "CancelApproval"
)

func approvePipelineRun(name, id string, pipeline *Pipeline, pipelineRun *PipelineRun) error {
	s := ""

	type pendingApproval struct {
		i     int
		name  string
		state string
	}

	var approvalsRequired []pendingApproval

	for i, stage := range pipeline.Stages {
		if !stage.Approval {
			continue
		}

		stageRun := pipelineRun.Stages[i]

		approval := stageRun.Metadata.Approval
		if approval.Name != "" {
			approvedBy := fmt.Sprintf("%s(%s)", approval.Name, approval.Email)
			s += "\n" + checkMark.Render() + " " + doneStyle.Render(fmt.Sprintf("Stage %d - %s already approved by %s\n", i+1, stageRun.Name, approvedBy))
		} else {
			approvalsRequired = append(approvalsRequired, pendingApproval{i: i, name: stageRun.Name, state: stageRun.State})
		}
	}

	if len(approvalsRequired) <= 0 {
		s += "\n" + currentStyle.Render(fmt.Sprintf("No pending approvals for pipeline %s with run id %s\n", name, id))
		fmt.Println(s)
		return nil
	}

	var options []huh.Option[string]

	for _, approval := range approvalsRequired {
		name := fmt.Sprintf("%s - %s", approval.name, approval.state)
		options = append(options, huh.NewOption(name, strconv.Itoa(approval.i)))
	}

	var approvals []string

	if err := huh.NewMultiSelect[string]().
		Options(
			options...,
		).
		Title("Approvals Required").
		Value(&approvals).Run(); err != nil {
		return err
	}

	if len(approvals) <= 0 {
		s += "\n" + crossMark.Render() + " " + failedStyle.Render("No additional stages approved\n")

		fmt.Println(s)
		return nil
	}

	cachedStore, err := users.GetCachedTokens()
	if err != nil {
		return err
	}

	approvedBy := fmt.Sprintf("%s(%s)", cachedStore.GithubUser.Login, cachedStore.GithubUser.Email)

	resource := map[string]string{"Pipeline": pipelineRun.PipelineName, "PipelineRun": pipelineRun.Id}
	for _, i := range approvals {
		stageNum, _ := strconv.Atoi(i)
		stage := pipelineRun.Stages[stageNum]

		s += "\n" + checkMark.Render() + " " + doneStyle.Render(fmt.Sprintf("Stage %d - %s approved by %s\n", stageNum+1, stage.Name, approvedBy))

		fmt.Println(s)

		pipelineRun.Stages[stageNum].Metadata.Approval = StageRunApproval{Name: cachedStore.GithubUser.Login, Email: cachedStore.GithubUser.Email}

		reason := fmt.Sprintf("Stage Approved %d - %s", stageNum, stage.Name)
		if err := audit.Save(context.Background(), AUDIT_APPROVED, resource, cachedStore.GithubUser.Login, cachedStore.GithubUser.Email, reason); err != nil {
			return err
		}
	}

	return savePipelineRun(context.Background(), pipelineRun)
}

func cancelApprovePipelineRun(name, id string, pipeline *Pipeline, pipelineRun *PipelineRun) error {
	s := ""

	type alreadyApproved struct {
		i     int
		name  string
		state string
	}

	var alreadyApprovedStages []alreadyApproved

	for i, stage := range pipeline.Stages {
		if !stage.Approval {
			continue
		}

		stageRun := pipelineRun.Stages[i]

		approval := stageRun.Metadata.Approval
		if approval.Name != "" {
			if strings.EqualFold(stageRun.State, "PendingApproval") || stageRun.State == "" {
				alreadyApprovedStages = append(alreadyApprovedStages, alreadyApproved{i: i, name: stageRun.Name, state: stageRun.State})
				continue
			}

			approvedBy := fmt.Sprintf("%s(%s)", approval.Name, approval.Email)
			s += "\n" + crossMark.Render() + " " + failedStyle.Render(fmt.Sprintf("Stage %d - %s with state %s cannot be canceled, approved by %s\n", i+1, stageRun.Name, stageRun.State, approvedBy))
		}
	}

	if len(alreadyApprovedStages) <= 0 {
		s += "\n" + currentStyle.Render(fmt.Sprintf("No approved stages for pipeline %s with run id %s\n", name, id))
		fmt.Println(s)
		return nil
	}

	var options []huh.Option[string]

	for _, approval := range alreadyApprovedStages {
		name := fmt.Sprintf("%s - %s", approval.name, approval.state)
		options = append(options, huh.NewOption(name, strconv.Itoa(approval.i)))
	}

	var approvals []string

	if err := huh.NewMultiSelect[string]().
		Options(
			options...,
		).
		Title("Cancel Approvals").
		Value(&approvals).Run(); err != nil {
		return err
	}

	if len(approvals) <= 0 {
		s += "\n" + crossMark.Render() + " " + failedStyle.Render("No additional stages approval canceled\n")

		fmt.Println(s)
		return nil
	}

	cachedStore, err := users.GetCachedTokens()
	if err != nil {
		return err
	}

	resource := map[string]string{"Pipeline": pipelineRun.PipelineName, "PipelineRun": pipelineRun.Id}
	for _, i := range approvals {
		stageNum, _ := strconv.Atoi(i)
		stage := pipelineRun.Stages[stageNum]

		s += "\n" + checkMark.Render() + " " + doneStyle.Render(fmt.Sprintf("Stage %d - %s canceled approval\n", stageNum+1, stage.Name))

		fmt.Println(s)

		pipelineRun.Stages[stageNum].Metadata.Approval = StageRunApproval{}

		reason := fmt.Sprintf("Canceled Approval for stage %d - %s", stageNum, stage.Name)
		if err := audit.Save(context.Background(), AUDIT_CANCEL_APPROVAL, resource, cachedStore.GithubUser.Login, cachedStore.GithubUser.Email, reason); err != nil {
			return err
		}
	}

	return savePipelineRun(context.Background(), pipelineRun)
}

func ApprovePipelineRunUI(name, id string) error {
	pipeline, err := GetPipeline(context.Background(), name)
	if err != nil {
		return err
	}

	if pipeline.Locked {
		resource := map[string]string{"Pipeline": pipeline.Name}
		latestAudit, err := audit.Latest(context.Background(), AUDIT_LOCKED, resource)
		if err != nil {
			return err
		}
		s := "\n" + crossMark.PaddingRight(1).Render() +
			failedStyle.Render("pipeline ") +
			warningStyle.Render(name) +
			failedStyle.Render(" locked at ") +
			warningStyle.Render(latestAudit.Time.String()) +
			failedStyle.Render(" by ") +
			warningStyle.Render(fmt.Sprintf("%s(%s) ", latestAudit.Actor, latestAudit.Email)) +
			warningStyle.Render(fmt.Sprintf(" due to %s ", latestAudit.Message)) + "\n"
		fmt.Println(s)
		return nil
	}

	pipelineRun, err := GetPipelineRun(context.Background(), name, id)
	if err != nil {
		return err
	}

	return approvePipelineRun(name, id, pipeline, pipelineRun)
}

func CancelApprovePipelineRunUI(name, id string) error {
	pipeline, err := GetPipeline(context.Background(), name)
	if err != nil {
		return err
	}

	pipelineRun, err := GetPipelineRun(context.Background(), name, id)
	if err != nil {
		return err
	}

	return cancelApprovePipelineRun(name, id, pipeline, pipelineRun)
}

func ApprovePipelineRun(ctx context.Context, name, id string, stageNum int) error {
	pipeline, err := GetPipeline(ctx, name)
	if err != nil {
		return err
	}

	if pipeline.Locked {
		resource := map[string]string{"Pipeline": pipeline.Name}
		latestAudit, err := audit.Latest(ctx, AUDIT_LOCKED, resource)
		if err != nil {
			return fmt.Errorf("Pipeline locked at %s, by %s(%s) due to %s", latestAudit.Time.String(), latestAudit.Actor, latestAudit.Email, latestAudit.Message)
		}
		return nil
	}

	pipelineRun, err := GetPipelineRun(ctx, name, id)
	if err != nil {
		return err
	}

	if stageNum < 0 && stageNum >= len(pipelineRun.Stages) {
		return fmt.Errorf("%d invalid stage, choose between 0 and %d", stageNum, len(pipelineRun.Stages)-1)
	}

	if pipelineRun.Stages[stageNum].Metadata.Approval.Name != "" {
		return nil
	}

	userName := ctx.Value(users.NameCtx).(string)
	userEmail := ctx.Value(users.EmailCtx).(string)

	pipelineRun.Stages[stageNum].Metadata.Approval = StageRunApproval{Name: userName, Email: userEmail}

	resource := map[string]string{"Pipeline": pipelineRun.PipelineName, "PipelineRun": pipelineRun.Id}
	reason := fmt.Sprintf("Stage Approved %d - %s", stageNum, pipelineRun.Stages[stageNum].Name)
	if err := audit.Save(ctx, AUDIT_APPROVED, resource, userName, userEmail, reason); err != nil {
		return err
	}

	return savePipelineRun(ctx, pipelineRun)
}

func CancelApprovePipelineRun(ctx context.Context, name, id string, stageNum int) error {
	pipelineRun, err := GetPipelineRun(ctx, name, id)
	if err != nil {
		return err
	}

	if stageNum < 0 && stageNum >= len(pipelineRun.Stages) {
		return fmt.Errorf("%d invalid stage, choose between 0 and %d", stageNum, len(pipelineRun.Stages)-1)
	}

	if pipelineRun.Stages[stageNum].Metadata.Approval.Name == "" {
		return nil
	}

	if pipelineRun.Stages[stageNum].Metadata.Approval.Name == "" {
		return nil
	}

	userName := ctx.Value(users.NameCtx).(string)
	userEmail := ctx.Value(users.EmailCtx).(string)

	resource := map[string]string{"Pipeline": pipelineRun.PipelineName, "PipelineRun": pipelineRun.Id}
	reason := fmt.Sprintf("Canceled Approval for stage %d - %s", stageNum, pipelineRun.Stages[stageNum].Name)
	if err := audit.Save(ctx, AUDIT_APPROVED, resource, userName, userEmail, reason); err != nil {
		return err
	}

	pipelineRun.Stages[stageNum].Metadata.Approval = StageRunApproval{}

	return savePipelineRun(ctx, pipelineRun)
}
