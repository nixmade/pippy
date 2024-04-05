package orgs

import (
	"context"
	"fmt"

	"github.com/nixmade/pippy/github"

	"github.com/urfave/cli/v3"
)

func Command() *cli.Command {
	return &cli.Command{
		Name:  "org",
		Usage: "org management",
		Commands: []*cli.Command{
			{
				Name:  "list",
				Usage: "list orgs for authenticated user",
				Action: func(ctx context.Context, c *cli.Command) error {
					if err := RunListOrgs(github.DefaultClient); err != nil {
						fmt.Printf("%v\n", err)
						return err
					}
					return nil
				},
			},
		},
	}
}
