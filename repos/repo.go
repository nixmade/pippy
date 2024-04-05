package repos

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/nixmade/pippy/github"

	"github.com/urfave/cli/v3"
)

func Command() *cli.Command {
	return &cli.Command{
		Name:  "repo",
		Usage: "repo management",
		Commands: []*cli.Command{
			{
				Name:  "list",
				Usage: "list repos in a org",
				Action: func(ctx context.Context, c *cli.Command) error {
					if err := RunListRepos(github.DefaultClient, c.String("type")); err != nil {
						fmt.Printf("%v\n", err)
						return err
					}
					return nil
				},
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:     "type",
						Usage:    "repo type all, owner, public, private, member",
						Value:    "owner",
						Required: false,
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
