package main

import (
	"context"

	"github.com/urfave/cli/v3"

	"github.com/sourcegraph/sourcegraph/cmd/embeddings/qa"
	"github.com/sourcegraph/sourcegraph/dev/sg/internal/category"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

var contextCommand = &cli.Command{
	Name:        "embeddings-qa",
	Usage:       "Calculate recall for embeddings",
	Description: "Recall is the fraction of relevant documents that were successfully retrieved. Recall=1 if, for every query in the test data, all relevant documents were retrieved. The command requires a running embeddings service with embeddings of the Sourcegraph repository.",
	Category:    category.Dev,
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:    "url",
			Value:   "http://127.0.0.1:9991/search",
			Aliases: []string{"u"},
			Usage:   "Run the evaluation against this endpoint",
		},
	},
	Action: func(ctx context.Context, cmd *cli.Command) error {
		url := cmd.String("url")
		if url == "" {
			return errors.New("url is empty")
		}

		_, err := qa.Run(qa.NewClient(url))

		return err
	},
}
