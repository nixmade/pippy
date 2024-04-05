package pipelines

import (
	"context"
	"fmt"

	"github.com/nixmade/pippy/audit"
	"github.com/nixmade/pippy/users"
)

const (
	AUDIT_LOCKED   string = "Locked"
	AUDIT_UNLOCKED string = "Unlocked"
)

func lockUnlockPipelineRun(pipeline *Pipeline, reason string, lock bool) error {
	userStore, err := users.GetCachedTokens()
	if err != nil {
		return err
	}

	resource := map[string]string{"Pipeline": pipeline.Name}
	if lock {
		pipeline.Locked = true
		if err := audit.Save(context.Background(), AUDIT_LOCKED, resource, userStore.GithubUser.Login, userStore.GithubUser.Email, reason); err != nil {
			return err
		}
		fmt.Println("\n" + checkMark.Render() + " " + doneStyle.Render("Successfully locked pipeline\n"))
	} else {
		pipeline.Locked = false
		if err := audit.Save(context.Background(), AUDIT_UNLOCKED, resource, userStore.GithubUser.Login, userStore.GithubUser.Email, reason); err != nil {
			return err
		}
		fmt.Println("\n" + checkMark.Render() + " " + doneStyle.Render("Successfully unlocked pipeline\n"))
	}

	return SavePipeline(context.Background(), pipeline)
}

func LockPipelineUI(name, reason string) error {
	pipeline, err := GetPipeline(context.Background(), name)
	if err != nil {
		return err
	}

	if pipeline.Locked {
		s := "\n" + crossMark.PaddingRight(1).Render() +
			failedStyle.Render("pipeline ") +
			warningStyle.Render(name) +
			failedStyle.Render(" already locked") + "\n"
		fmt.Println(s)
		return nil
	}

	return lockUnlockPipelineRun(pipeline, reason, true)
}

func UnlockPipelineUI(name, reason string) error {
	pipeline, err := GetPipeline(context.Background(), name)
	if err != nil {
		return err
	}

	if !pipeline.Locked {
		s := "\n" + crossMark.PaddingRight(1).Render() +
			failedStyle.Render("pipeline ") +
			warningStyle.Render(name) +
			failedStyle.Render(" not locked\n")
		fmt.Println(s)
		return nil
	}

	return lockUnlockPipelineRun(pipeline, reason, false)
}

func LockPipeline(ctx context.Context, name, reason string) error {
	pipeline, err := GetPipeline(ctx, name)
	if err != nil {
		return err
	}

	if pipeline.Locked {
		return nil
	}

	userName := ctx.Value(users.NameCtx).(string)
	userEmail := ctx.Value(users.EmailCtx).(string)

	resource := map[string]string{"Pipeline": pipeline.Name}
	pipeline.Locked = true
	if err := audit.Save(ctx, AUDIT_LOCKED, resource, userName, userEmail, reason); err != nil {
		return err
	}

	return SavePipeline(ctx, pipeline)
}

func UnlockPipeline(ctx context.Context, name, reason string) error {
	pipeline, err := GetPipeline(ctx, name)
	if err != nil {
		return err
	}

	if !pipeline.Locked {
		return nil
	}

	userName := ctx.Value(users.NameCtx).(string)
	userEmail := ctx.Value(users.EmailCtx).(string)

	resource := map[string]string{"Pipeline": pipeline.Name}
	pipeline.Locked = false
	if err := audit.Save(ctx, AUDIT_UNLOCKED, resource, userName, userEmail, reason); err != nil {
		return err
	}

	return SavePipeline(ctx, pipeline)
}
