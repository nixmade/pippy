package workflows

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/nixmade/pippy/github"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/huh"
	"github.com/urfave/cli/v3"
)

func Command() *cli.Command {
	return &cli.Command{
		Name:  "workflow",
		Usage: "workflow management",
		Commands: []*cli.Command{
			{
				Name:  "list",
				Usage: "list workflows for a repo",
				Action: func(ctx context.Context, c *cli.Command) error {
					if err := RunListWorkflows(github.DefaultClient, c.String("type")); err != nil {
						fmt.Printf("%v\n", err)
						return err
					}
					return nil
				},
				Flags: []cli.Flag{
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
				Name:  "validate",
				Usage: "checks if the repo has correct configuration for pippy to function correctly",
				Action: func(ctx context.Context, c *cli.Command) error {
					if err := RunValidateRepoWorkflows(github.DefaultClient, c.String("type")); err != nil {
						fmt.Printf("%v\n", err)
						return err
					}
					return nil
				},
				Flags: []cli.Flag{
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
		},
	}
}

func RunListWorkflows(c github.Client, repoType string) error {
	var orgRepo string
	repos, err := GetRepos(repoType)
	if err != nil {
		return err
	}
	if err := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Options(huh.NewOptions(repos...)...).
				Title("Choose a repo").
				Description("workflows for selected repo").
				Value(&orgRepo),
		)).Run(); err != nil {
		return err
	}

	orgRepoSlice := strings.SplitN(orgRepo, "/", 2)
	org := orgRepoSlice[0]
	repo := orgRepoSlice[1]
	workflowItems, err := c.ListWorkflows(org, repo)
	if err != nil {
		return err
	}

	var listItems []list.Item
	for _, workflowItem := range workflowItems {
		listItems = append(listItems, workflowItem)
	}

	return Run(org, repo, listItems)
}
