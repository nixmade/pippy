package pipelines

import (
	"context"
	"fmt"

	"github.com/nixmade/pippy/audit"
	"github.com/nixmade/pippy/users"
)

const (
	AUDIT_PAUSED  string = "Paused"
	AUDIT_RESUMED string = "Resumed"
)

func pauseResumePipelineRun(pipelineRun *PipelineRun, reason string, pause bool) error {
	userStore, err := users.GetCachedTokens()
	if err != nil {
		return err
	}

	resource := map[string]string{"Pipeline": pipelineRun.PipelineName, "PipelineRun": pipelineRun.Id}
	if pause {
		pipelineRun.Paused = true

		if err := audit.Save(context.Background(), AUDIT_PAUSED, resource, userStore.GithubUser.Login, userStore.GithubUser.Email, reason); err != nil {
			return err
		}
		fmt.Println("\n" + checkMark.Render() + " " + doneStyle.Render("Successfully paused pipeline\n"))
	} else {
		pipelineRun.Paused = false
		latestAudit, err := audit.Latest(context.Background(), AUDIT_PAUSED, resource)
		if err != nil {
			return err
		}
		s := "\n" + checkMark.Render() + " " +
			doneStyle.Render("Successfully resumed pipeline paused at ") +
			warningStyle.Render(latestAudit.Time.String()) +
			doneStyle.Render(" by ") +
			warningStyle.Render(fmt.Sprintf("%s(%s) ", latestAudit.Actor, latestAudit.Email)) +
			doneStyle.Render(" - ") + "\"" +
			warningStyle.Render(latestAudit.Message) + "\"\n"
		fmt.Println(s)

		if err := audit.Save(context.Background(), AUDIT_RESUMED, resource, userStore.GithubUser.Login, userStore.GithubUser.Email, reason); err != nil {
			return err
		}
	}

	return savePipelineRun(context.Background(), pipelineRun)
}

func PausePipelineRunUI(name, id, reason string) error {
	pipelineRun, err := GetPipelineRun(context.Background(), name, id)
	if err != nil {
		return err
	}

	if pipelineRun.Paused {
		resource := map[string]string{"Pipeline": pipelineRun.PipelineName, "PipelineRun": pipelineRun.Id}
		latestAudit, err := audit.Latest(context.Background(), AUDIT_PAUSED, resource)
		if err != nil {
			return err
		}
		s := "\n" + crossMark.PaddingRight(1).Render() +
			failedStyle.Render("pipeline ") +
			warningStyle.Render(name) +
			failedStyle.Render(" already paused at ") +
			warningStyle.Render(latestAudit.Time.String()) +
			failedStyle.Render(" by ") +
			warningStyle.Render(fmt.Sprintf("%s(%s) ", latestAudit.Actor, latestAudit.Email)) +
			failedStyle.Render("- ") + "\"" +
			warningStyle.Render(latestAudit.Message) + "\"\n"
		fmt.Println(s)
		return nil
	}

	return pauseResumePipelineRun(pipelineRun, reason, true)
}

func ResumePipelineRunUI(name, id, reason string) error {
	pipelineRun, err := GetPipelineRun(context.Background(), name, id)
	if err != nil {
		return err
	}

	if !pipelineRun.Paused {
		s := "\n" + crossMark.PaddingRight(1).Render() +
			failedStyle.Render("pipeline ") +
			warningStyle.Render(name) +
			failedStyle.Render(" not paused\n")
		fmt.Println(s)
		return nil
	}

	return pauseResumePipelineRun(pipelineRun, reason, false)
}

func PausePipelineRun(ctx context.Context, name, id, reason string) error {
	pipelineRun, err := GetPipelineRun(ctx, name, id)
	if err != nil {
		return err
	}

	if pipelineRun.Paused {
		return nil
	}

	userName := ctx.Value(users.NameCtx).(string)
	userEmail := ctx.Value(users.EmailCtx).(string)
	resource := map[string]string{"Pipeline": pipelineRun.PipelineName, "PipelineRun": pipelineRun.Id}
	pipelineRun.Paused = true

	if err := audit.Save(ctx, AUDIT_PAUSED, resource, userName, userEmail, reason); err != nil {
		return err
	}

	return savePipelineRun(ctx, pipelineRun)
}

func ResumePipelineRun(ctx context.Context, name, id, reason string) error {
	pipelineRun, err := GetPipelineRun(ctx, name, id)
	if err != nil {
		return err
	}

	if !pipelineRun.Paused {
		return nil
	}

	userName := ctx.Value(users.NameCtx).(string)
	userEmail := ctx.Value(users.EmailCtx).(string)
	resource := map[string]string{"Pipeline": pipelineRun.PipelineName, "PipelineRun": pipelineRun.Id}
	pipelineRun.Paused = false
	if err := audit.Save(ctx, AUDIT_RESUMED, resource, userName, userEmail, reason); err != nil {
		return err
	}
	return savePipelineRun(ctx, pipelineRun)
}
