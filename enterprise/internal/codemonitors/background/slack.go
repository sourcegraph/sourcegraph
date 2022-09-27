package background

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"code.gitea.io/gitea/modules/hostmatcher"
	"github.com/slack-go/slack"

	"github.com/sourcegraph/sourcegraph/internal/search/result"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

func sendSlackNotification(ctx context.Context, url string, args actionArgs) error {
	return postSlackWebhook(ctx, url, slackPayload(args))
}

func slackPayload(args actionArgs) *slack.WebhookMessage {
	newMarkdownSection := func(s string) slack.Block {
		return slack.NewSectionBlock(slack.NewTextBlockObject("mrkdwn", s, false, false), nil, nil)
	}

	truncatedResults, totalCount, truncatedCount := truncateResults(args.Results, 5)

	blocks := []slack.Block{
		newMarkdownSection(fmt.Sprintf(
			"%s's Sourcegraph Code monitor, *%s*, detected *%d* new matches.",
			args.MonitorOwnerName,
			args.MonitorDescription,
			totalCount,
		)),
	}

	if args.IncludeResults {
		for _, result := range truncatedResults {
			resultType := "Message"
			if result.DiffPreview != nil {
				resultType = "Diff"
			}
			blocks = append(blocks, newMarkdownSection(fmt.Sprintf(
				"%s match: <%s|%s@%s>",
				resultType,
				getCommitURL(args.ExternalURL, string(result.Repo.Name), string(result.Commit.ID), args.UTMSource),
				result.Repo.Name,
				result.Commit.ID.Short(),
			)))
			var contentRaw string
			if result.DiffPreview != nil {
				contentRaw = truncateString(result.DiffPreview.Content, 10)
			} else {
				contentRaw = truncateString(result.MessagePreview.Content, 10)
			}
			blocks = append(blocks, newMarkdownSection(formatCodeBlock(contentRaw)))
		}
		if truncatedCount > 0 {
			blocks = append(blocks, newMarkdownSection(fmt.Sprintf(
				"...and <%s|%d more matches>.",
				getSearchURL(args.ExternalURL, args.Query, args.UTMSource),
				truncatedCount,
			)))
		}
	} else {
		blocks = append(blocks, newMarkdownSection(fmt.Sprintf(
			"<%s|View results>",
			getSearchURL(args.ExternalURL, args.Query, args.UTMSource),
		)))
	}

	blocks = append(blocks,
		newMarkdownSection(fmt.Sprintf(
			`If you are %s, you can <%s|edit your code monitor>`,
			args.MonitorOwnerName,
			getCodeMonitorURL(args.ExternalURL, args.MonitorID, args.UTMSource),
		)),
	)
	return &slack.WebhookMessage{Blocks: &slack.Blocks{BlockSet: blocks}}
}

func formatCodeBlock(s string) string {
	return fmt.Sprintf("```%s```", strings.ReplaceAll(s, "```", "\\`\\`\\`"))
}

func truncateString(input string, lines int) string {
	splitLines := strings.SplitAfter(input, "\n")
	if len(splitLines) > lines {
		splitLines = splitLines[:lines]
		splitLines = append(splitLines, "...\n")
	}
	return strings.Join(splitLines, "")
}

func truncateResults(results []*result.CommitMatch, maxResults int) (_ []*result.CommitMatch, totalCount, truncatedCount int) {
	// Convert to type result.Matches
	matches := make(result.Matches, len(results))
	for i, res := range results {
		matches[i] = res
	}

	totalCount = matches.ResultCount()
	matches.Limit(maxResults)
	outputCount := matches.ResultCount()

	// Convert back type []*result.CommitMatch
	output := make([]*result.CommitMatch, len(matches))
	for i, match := range matches {
		output[i] = match.(*result.CommitMatch)
	}

	return output, totalCount, totalCount - outputCount
}

// adapted from slack.PostWebhookCustomHTTPContext
func postSlackWebhook(ctx context.Context, url string, msg *slack.WebhookMessage) error {

	raw, err := json.Marshal(msg)
	if err != nil {
		return errors.Wrap(err, "marshal failed")
	}

	// Create an allowList out of specified HostList
	hostList := os.Getenv("WEBHOOK_ALLOWLIST")
	allowList := hostmatcher.ParseHostMatchList("", hostList)

	webHookHttpClient := &http.Client{
		Transport: &http.Transport{
			DialContext: hostmatcher.NewDialContext("code-monitor-slackwebhook", allowList, nil),
		},
	}
	resp, err := webHookHttpClient.Post(url, "application/json", bytes.NewReader(raw))
	if err != nil {
		return errors.Wrap(err, "failed to post webhook")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return StatusCodeError{
			Code:   resp.StatusCode,
			Status: resp.Status,
			Body:   string(body),
		}
	}

	return nil
}

func SendTestSlackWebhook(ctx context.Context, description, url string) error {
	testMessage := &slack.WebhookMessage{Blocks: &slack.Blocks{BlockSet: []slack.Block{
		slack.NewSectionBlock(
			slack.NewTextBlockObject("mrkdwn",
				fmt.Sprintf(
					"Test message for Code Monitor '%s'",
					description,
				),
				false,
				false,
			),
			nil,
			nil,
		),
	}}}

	return postSlackWebhook(ctx, url, testMessage)
}
