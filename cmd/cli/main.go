package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"sort"

	"github.com/nixmade/pippy/audit"
	"github.com/nixmade/pippy/orgs"
	"github.com/nixmade/pippy/pipelines"
	"github.com/nixmade/pippy/repos"
	"github.com/nixmade/pippy/users"
	"github.com/nixmade/pippy/workflows"

	"github.com/urfave/cli/v3"
)

var (
	version = "1.0.0-beta"
	// commit  = "none"
	// date    = "unknown"
)

func main() {
	cli.VersionFlag = &cli.BoolFlag{
		Name:    "print-version",
		Aliases: []string{"V"},
		Usage:   "print only the version",
	}
	appCli := &cli.Command{
		Name:    "pippy",
		Version: fmt.Sprintf("v%s", version),
		Usage:   "pippy interacts with github actions",
		Commands: []*cli.Command{
			users.Command(),
			workflows.Command(),
			repos.Command(),
			orgs.Command(),
			pipelines.Command(),
			audit.Command(),
		},
	}

	sort.Sort(cli.FlagsByName(appCli.Flags))

	if err := appCli.Run(context.Background(), os.Args); err != nil {
		log.Fatal(err)
	}
}
