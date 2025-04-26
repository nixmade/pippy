package pipelines

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/urfave/cli/v3"
)

const (
	PipelinePrefix    = "pipeline:"
	PipelineRunPrefix = "pipelinerun:"
)

func Command() *cli.Command {
	return &cli.Command{
		Name:  "pipeline",
		Usage: "workflow management",
		Commands: []*cli.Command{
			{
				Name:  "create",
				Usage: "create pipelines from workflows across single or multiple repos",
				Action: func(ctx context.Context, c *cli.Command) error {
					if err := CreatePipeline(c.String("name"), c.String("type")); err != nil {
						fmt.Printf("%v\n", err)
						return err
					}
					return nil
				},
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:     "name",
						Usage:    "pipeline name",
						Required: true,
					},
					&cli.StringFlag{
						Name:     "type",
						Usage:    "repo type all, owner, public, private, member",
						Required: false,
						Value:    "owner",
						Action: func(ctx context.Context, c *cli.Command, v string) error {
							validValues := []string{"all", "owner", "public", "private", "member"}
							if slices.Contains(validValues, v) {
								return nil
							}
							return fmt.Errorf("please provide a valid value in %s", strings.Join(validValues, ","))
						},
					},
				},
			},
			{
				Name:  "list",
				Usage: "list pipeline already saved",
				Action: func(ctx context.Context, c *cli.Command) error {
					if err := ShowAllPipelines(); err != nil {
						fmt.Printf("%v\n", err)
						return err
					}
					return nil
				},
			},
			{
				Name:  "delete",
				Usage: "delete pipeline already saved",
				Action: func(ctx context.Context, c *cli.Command) error {
					if err := DeletePipeline(context.Background(), c.String("name")); err != nil {
						fmt.Printf("%v\n", err)
						return err
					}
					fmt.Println("\n" + checkMark.Render() + " " + doneStyle.Render("Successfully deleted pipeline\n"))
					return nil
				},
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:     "name",
						Usage:    "pipeline name",
						Required: true,
					},
				},
			},
			{
				Name:  "show",
				Usage: "show pipeline already saved",
				Action: func(ctx context.Context, c *cli.Command) error {
					if err := ShowPipeline(c.String("name")); err != nil {
						fmt.Printf("%v\n", err)
						return err
					}
					return nil
				},
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:     "name",
						Usage:    "pipeline name",
						Required: true,
					},
				},
			},
			{
				Name:  "lock",
				Usage: "lock pipeline to deny all approvals",
				Action: func(ctx context.Context, c *cli.Command) error {
					if err := LockPipelineUI(c.String("name"), c.String("reason")); err != nil {
						fmt.Printf("%v\n", err)
						return err
					}
					return nil
				},
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:     "name",
						Usage:    "pipeline name",
						Required: true,
					},
					&cli.StringFlag{
						Name:     "reason",
						Usage:    "lock reason",
						Required: true,
					},
				},
			},
			{
				Name:  "unlock",
				Usage: "unlock pipeline to allow all approvals",
				Action: func(ctx context.Context, c *cli.Command) error {
					if err := UnlockPipelineUI(c.String("name"), c.String("reason")); err != nil {
						fmt.Printf("%v\n", err)
						return err
					}
					return nil
				},
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:     "name",
						Usage:    "pipeline name",
						Required: true,
					},
					&cli.StringFlag{
						Name:     "reason",
						Usage:    "lock reason",
						Required: true,
					},
				},
			},
			{
				Name:  "run",
				Usage: "pipeline runs",
				Commands: []*cli.Command{
					{
						Name:  "execute",
						Usage: "run pipeline",
						Action: func(ctx context.Context, c *cli.Command) error {
							inputs := c.StringSlice("input")
							inputPair := parseKeyValuePairs(inputs)
							if err := RunPipelineUI(c.String("name"), c.String("id"), inputPair, c.Bool("force")); err != nil {
								fmt.Printf("%v\n", err)
								return err
							}
							return nil
						},
						Flags: []cli.Flag{
							&cli.StringFlag{
								Name:     "name",
								Usage:    "pipeline name",
								Required: true,
							},
							&cli.StringSliceFlag{
								Name:     "input",
								Usage:    "pipeline input provided as kv pair, --input version=44ffae --input description='detailed here'",
								Required: false,
							},
							&cli.StringFlag{
								Name:     "id",
								Usage:    "pipeline run id to resume, blank for a new run",
								Value:    "",
								Required: false,
							},
							&cli.BoolFlag{
								Name:     "force",
								Usage:    "use with caution, verify there isnt any other pipeline run in progress, force current version to run",
								Value:    false,
								Required: false,
							},
						},
					},
					{
						Name:  "list",
						Usage: "show pipeline runs",
						Action: func(ctx context.Context, c *cli.Command) error {
							if err := ShowAllPipelineRuns(c.String("name"), c.Int64("count")); err != nil {
								fmt.Printf("%v\n", err)
								return err
							}
							return nil
						},
						Flags: []cli.Flag{
							&cli.StringFlag{
								Name:     "name",
								Usage:    "pipeline name",
								Required: true,
							},
							&cli.Int64Flag{
								Name:     "count",
								Usage:    "latest n pipeline runs, -1/0 for all",
								Value:    10,
								Required: false,
							},
						},
					},
					{
						Name:  "show",
						Usage: "show pipeline run details",
						Action: func(ctx context.Context, c *cli.Command) error {
							if err := ShowPipelineRun(c.String("name"), c.String("id")); err != nil {
								fmt.Printf("%v\n", err)
								return err
							}
							return nil
						},
						Flags: []cli.Flag{
							&cli.StringFlag{
								Name:     "name",
								Usage:    "pipeline name",
								Required: true,
							},
							&cli.StringFlag{
								Name:     "id",
								Usage:    "pipeline run id",
								Required: true,
							},
						},
					},
					{
						Name:  "approve",
						Usage: "approve pipeline run for stage pending approval",
						Action: func(ctx context.Context, c *cli.Command) error {
							if err := ApprovePipelineRunUI(c.String("name"), c.String("id")); err != nil {
								fmt.Printf("%v\n", err)
								return err
							}
							return nil
						},
						Flags: []cli.Flag{
							&cli.StringFlag{
								Name:     "name",
								Usage:    "pipeline name",
								Required: true,
							},
							&cli.StringFlag{
								Name:     "id",
								Usage:    "pipeline run id",
								Required: true,
							},
						},
					},
					{
						Name:  "cancel",
						Usage: "cancel approval of pipeline run for approved stage",
						Action: func(ctx context.Context, c *cli.Command) error {
							if err := CancelApprovePipelineRunUI(c.String("name"), c.String("id")); err != nil {
								fmt.Printf("%v\n", err)
								return err
							}
							return nil
						},
						Flags: []cli.Flag{
							&cli.StringFlag{
								Name:     "name",
								Usage:    "pipeline name",
								Required: true,
							},
							&cli.StringFlag{
								Name:     "id",
								Usage:    "pipeline run id",
								Required: true,
							},
						},
					},
					{
						Name:  "pause",
						Usage: "pause pipeline run",
						Action: func(ctx context.Context, c *cli.Command) error {
							if err := PausePipelineRunUI(c.String("name"), c.String("id"), c.String("reason")); err != nil {
								fmt.Printf("%v\n", err)
								return err
							}
							return nil
						},
						Flags: []cli.Flag{
							&cli.StringFlag{
								Name:     "name",
								Usage:    "pipeline name",
								Required: true,
							},
							&cli.StringFlag{
								Name:     "id",
								Usage:    "pipeline run id",
								Required: true,
							},
							&cli.StringFlag{
								Name:     "reason",
								Usage:    "pipeline pause reason",
								Required: true,
							},
						},
					},
					{
						Name:  "resume",
						Usage: "resume pipeline run",
						Action: func(ctx context.Context, c *cli.Command) error {
							if err := ResumePipelineRunUI(c.String("name"), c.String("id"), c.String("reason")); err != nil {
								fmt.Printf("%v\n", err)
								return err
							}
							return nil
						},
						Flags: []cli.Flag{
							&cli.StringFlag{
								Name:     "name",
								Usage:    "pipeline name",
								Required: true,
							},
							&cli.StringFlag{
								Name:     "id",
								Usage:    "pipeline run id",
								Required: true,
							},
							&cli.StringFlag{
								Name:     "reason",
								Usage:    "pipeline resume reason",
								Required: true,
							},
						},
					},
				},
			},
		},
	}
}

func parseKeyValuePairs(kvPairs []string) map[string]string {
	kvMap := make(map[string]string)
	for _, pair := range kvPairs {
		parts := strings.SplitN(pair, "=", 2)
		if len(parts) == 2 {
			kvMap[parts[0]] = parts[1]
		} else {
			kvMap[parts[0]] = ""
		}
	}
	return kvMap
}
