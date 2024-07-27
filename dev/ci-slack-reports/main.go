package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"cloud.google.com/go/bigquery"
	"github.com/olekukonko/tablewriter"
	"github.com/sourcegraph/log"
	"golang.org/x/oauth2"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"

	"github.com/slack-go/slack"

	"github.com/sourcegraph/sourcegraph/internal/version"
	"github.com/sourcegraph/sourcegraph/lib/managedservicesplatform/runtime/contract"
)

func main() {
	ctx := context.Background()
	flag.Parse()

	liblog := log.Init(log.Resource{
		Name: "ci-slack-reports",
	}, log.NewSentrySink())
	defer liblog.Sync()
	logger := log.Scoped("run")

	env, err := contract.ParseEnv(os.Environ())
	if err != nil {
		logger.Fatal("failed to parse env", log.Error(err))
	}

	contract := contract.NewJob(logger.Scoped("msp"), job{}, env)
	if contract.Diagnostics.ConfigureSentry(liblog) {
		logger.Info("Sentry integration enabled")
	}

	config := new(Config)
	config.Load(env)

	slackClient := slack.New(config.SlackToken)

	// for testing only
	token := oauth2.Token{
		AccessToken: os.Getenv("GCP_TOKEN"),
	}
	bq, err := bigquery.NewClient(ctx, config.BigQueryProjectID, option.WithTokenSource(oauth2.StaticTokenSource(&token)))
	if err != nil {
		logger.Fatal("failed to create BigQuery client", log.Error(err))
	}
	defer bq.Close()

	endTime := time.Now().Truncate(time.Hour * 24)
	startTime := endTime.Add((-time.Hour * 24 * 7) * time.Duration(config.LookbackWindowWeeks))

	const n = 6

	rows, err := runQuery(ctx, *bq, topNSumTime, []bigquery.QueryParameter{
		{
			Name:  "start_time",
			Value: startTime,
		},
		{
			Name:  "end_time",
			Value: endTime,
		},
		{
			Name:  "n",
			Value: n,
		},
	})
	if err != nil {
		logger.Fatal("error fetching top "+strconv.Itoa(n), log.Error(err))
	}

	type Row struct {
		Target    string `bigquery:"target"`
		LastOwner string `bigquery:"last_owner"`
		// TotalTime in the week in minutes
		TotalTime  float32 `bigquery:"total_time"`
		CacheRatio float32 `bigquery:"cache_ratio"`
		Runs       int     `bigquery:"runs"`
	}

	teamToTests := make(map[string][]Row)

	for {
		var row Row
		err := rows.Next(&row)
		if err == iterator.Done {
			break
		}
		if err != nil {
			logger.Fatal("failed to read query results", log.Error(err))
		}

		teamToTests[row.LastOwner] = append(teamToTests[row.LastOwner], row)
	}

	for team, tests := range teamToTests {
		channel, ok := config.TeamChannelMapping[team]
		if !ok {
			channel = DefaultChannel
		}

		message := fmt.Sprintf(":bazel: *Your team's top %d tests with most CI time in the past %d week(s)* _for %s_\n", len(tests), config.LookbackWindowWeeks, strings.ReplaceAll(strings.TrimPrefix(team, "owner_"), "_", " "))
		message += "See <https://www.notion.so/sourcegraph/Understanding-Bazel-test-ownership-and-CI-impact-64b02df8f7934df9b908bf464c120847#c00955bfc98d478a9504f190139bb855|the Notion doc> for more information."

		var out strings.Builder
		table := tablewriter.NewWriter(&out)
		table.SetHeader([]string{"Target", "Time", "Cache %", "Runs"})

		for _, test := range tests {
			duration, _ := time.ParseDuration(strconv.FormatFloat(float64(test.TotalTime), 'f', 2, 32) + "m")
			table.Append([]string{test.Target, duration.String(), strconv.Itoa(int(test.CacheRatio * 100)), strconv.Itoa(test.Runs)})
		}

		table.SetBorder(false)
		table.SetAutoWrapText(false)
		table.SetColWidth(80)
		table.Render()

		_, err := slackClient.UploadFile(slack.FileUploadParameters{
			Content:        out.String(),
			InitialComment: message,
			Channels:       []string{channel},
		})
		if err != nil {
			logger.Error("failed to upload a report", log.Error(err), log.String("team", team), log.String("channel", channel))
		}
	}
}

type job struct{}

func (s job) Name() string { return "ci-slack-reports" }

func (s job) Version() string { return version.Version() }
