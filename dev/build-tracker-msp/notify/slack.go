package notify

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/google/go-github/v55/github"
	"github.com/slack-go/slack"
	"github.com/sourcegraph/log"

	"github.com/sourcegraph/sourcegraph/dev/team"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

const StepShowLimit = 5

type cacheItem[T any] struct {
	Value     T
	Timestamp time.Time
}

func newCacheItem[T any](value T) *cacheItem[T] {
	return &cacheItem[T]{
		Value:     value,
		Timestamp: time.Now(),
	}
}

type NotificationClient interface {
	Send(info *BuildNotification) error
	GetNotification(buildNumber int) *SlackNotification
}

type Client struct {
	slack   slack.Client
	team    team.TeammateResolver
	history map[int]*SlackNotification
	logger  log.Logger
	channel string
}

type BuildNotification struct {
	BuildNumber        int
	ConsecutiveFailure int
	PipelineName       string
	AuthorEmail        string
	Message            string
	Commit             string
	BuildURL           string
	BuildStatus        string
	Fixed              []JobLine
	Failed             []JobLine
	Passed             []JobLine
	TotalSteps         int
}

type JobLine interface {
	Title() string
	LogURL() string
}

type SlackNotification struct {
	// SentAt is the time the notification got sent.
	SentAt time.Time
	// ID is the unique idenfifier which represents this notification in Slack. Typically this is the timestamp as
	// is returned by the Slack API upon successful send of a notification.
	ID string
	// ChannelID is the channelID as returned by the Slack API after successful sending of a notification. It is NOT
	// the traditional channel you're use to that starts with a '#'. Instead it's the global ID for that channel used by
	// Slack.
	ChannelID string

	// BuildNotification is the BuildNotification that was used to send this SlackNotification
	BuildNotification *BuildNotification

	// AuthorMention is the author mention used for notify the teammate for this notification
	//
	// Ideally we should not store the mentionn but the actual Teammate. But a teammate
	AuthorMention string
}

func (n *SlackNotification) Equals(o *SlackNotification) bool {
	if o == nil {
		return false
	}

	return n.ID == o.ID && n.ChannelID == o.ChannelID && n.SentAt.Equal(o.SentAt)
}

func NewSlackNotification(id, channel string, info *BuildNotification, author string) *SlackNotification {
	return &SlackNotification{
		SentAt:            time.Now(),
		ID:                id,
		ChannelID:         channel,
		BuildNotification: info,
		AuthorMention:     author,
	}
}

func NewClient(logger log.Logger, slackToken, githubToken, channel string) *Client {
	debug := os.Getenv("BUILD_TRACKER_SLACK_DEBUG") == "1"
	slackClient := slack.New(slackToken, slack.OptionDebug(debug))

	httpClient := http.Client{
		Timeout: 5 * time.Second,
	}
	githubClient := github.NewClient(&httpClient)
	teamResolver := team.NewTeammateResolver(githubClient, slackClient)

	history := make(map[int]*SlackNotification)

	return &Client{
		logger:  logger.Scoped("notificationClient"),
		slack:   *slackClient,
		team:    teamResolver,
		channel: channel,
		history: history,
	}
}

func (c *Client) Send(info *BuildNotification) error {
	if prev := c.GetNotification(info.BuildNumber); prev != nil {
		if sent, err := c.sendUpdatedMessage(info, prev); err == nil {
			c.history[info.BuildNumber] = sent
		} else {
			return err
		}
	} else if sent, err := c.sendNewMessage(info); err != nil {
		return err
	} else {
		c.history[info.BuildNumber] = sent
	}

	return nil
}

func (c *Client) GetNotification(buildNumber int) *SlackNotification {
	notification, ok := c.history[buildNumber]
	if !ok {
		return nil
	}
	return notification
}

func (c *Client) sendUpdatedMessage(info *BuildNotification, previous *SlackNotification) (*SlackNotification, error) {
	if previous == nil {
		return nil, errors.New("cannot update message with nil notification")
	}
	logger := c.logger.With(log.Int("buildNumber", info.BuildNumber), log.String("channel", c.channel))
	logger.Debug("creating slack json")

	blocks := c.createMessageBlocks(info, previous.AuthorMention)
	// Slack responds with the message timestamp and a channel, which you have to use when you want to update the message.
	var id, channel string
	logger.Debug("sending updated notification")
	msgOptBlocks := slack.MsgOptionBlocks(blocks...)
	// Note: for UpdateMessage using the #channel-name format doesn't work, you need the Slack ChannelID.
	channel, id, _, err := c.slack.UpdateMessage(previous.ChannelID, previous.ID, msgOptBlocks)
	if err != nil {
		logger.Error("failed to update message", log.Error(err))
		return previous, err
	}

	return NewSlackNotification(id, channel, info, previous.AuthorMention), nil
}

func (c *Client) sendNewMessage(info *BuildNotification) (*SlackNotification, error) {
	logger := c.logger.With(log.Int("buildNumber", info.BuildNumber), log.String("channel", c.channel))
	logger.Debug("creating slack json")

	author := ""
	teammate, err := c.GetTeammateForCommit(info.Commit)
	if err != nil {
		c.logger.Error("failed to find teammate", log.Error(err))
		// the error has some guidance on how to fix it so that teammate resolver can figure out who you are from the commit!
		// so we set author here to that msg, so that the message can be conveyed to the person in slack
		author = err.Error()
	} else {
		logger.Debug("teammate found", log.Object("teammate",
			log.String("slackID", teammate.SlackID),
			log.String("key", teammate.Key),
			log.String("email", teammate.Email),
			log.String("handbook", teammate.HandbookLink),
			log.String("slackName", teammate.SlackName),
			log.String("github", teammate.GitHub),
		))
		author = SlackMention(teammate)
	}

	blocks := c.createMessageBlocks(info, author)
	// Slack responds with the message timestamp and a channel, which you have to use when you want to update the message.
	var id, channel string

	logger.Debug("sending new notification")
	msgOptBlocks := slack.MsgOptionBlocks(blocks...)
	channel, id, err = c.slack.PostMessage(c.channel, msgOptBlocks)
	if err != nil {
		logger.Error("failed to post message", log.Error(err))
		return nil, err
	}

	logger.Info("notification posted")
	return NewSlackNotification(id, channel, info, author), nil
}

func commitLink(msg, commit string) string {
	repo := "http://github.com/sourcegraph/sourcegraph"
	sgURL := fmt.Sprintf("%s/commit/%s", repo, commit)
	return fmt.Sprintf("<%s|%s>", sgURL, msg)
}

func SlackMention(teammate *team.Teammate) string {
	if teammate.SlackID == "" {
		return fmt.Sprintf("%s (%s) - We could not locate your Slack ID. Please check that your information in the Handbook team.yml file is correct", teammate.Name, teammate.Email)
	}
	return fmt.Sprintf("<@%s>", teammate.SlackID)
}

func createStepsSection(status string, items []JobLine, showLimit int) string {
	if len(items) == 0 {
		return ""
	}
	section := fmt.Sprintf("*%s jobs:*\n\n", status)
	// if there are more than JobShowLimit of failed jobs, we cannot print all of it
	// since the message will to big and slack will reject the message with "invalid_blocks"
	if len(items) > StepShowLimit {
		section = fmt.Sprintf("* %d %s jobs (showing %d):*\n\n", len(items), status, showLimit)
	}
	for i := range min(showLimit, len(items)) {
		item := items[i]

		line := fmt.Sprintf("● %s", item.Title())
		if item.LogURL() != "" {
			line += fmt.Sprintf("- <%s|logs>", item.LogURL())
		}
		line += "\n"
		section += line
	}

	return section + "\n"
}

func (c *Client) GetTeammateForCommit(commit string) (*team.Teammate, error) {
	result, err := c.team.ResolveByCommitAuthor(context.Background(), "sourcegraph", "sourcegraph", commit)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (c *Client) createMessageBlocks(info *BuildNotification, author string) []slack.Block {
	msg, _, _ := strings.Cut(info.Message, "\n")
	msg += fmt.Sprintf(" (%s)", info.Commit[:7])

	section := fmt.Sprintf("> %s\n\n", commitLink(msg, info.Commit))

	// create a bulleted list of all the failed jobs
	jobSection := createStepsSection("Fixed", info.Fixed, StepShowLimit)
	jobSection += createStepsSection("Failed", info.Failed, StepShowLimit)
	section += jobSection

	blocks := []slack.Block{
		slack.NewHeaderBlock(
			slack.NewTextBlockObject(slack.PlainTextType, generateSlackHeader(info), true, false),
		),
		slack.NewSectionBlock(&slack.TextBlockObject{Type: slack.MarkdownType, Text: section}, nil, nil),
		slack.NewSectionBlock(
			nil,
			[]*slack.TextBlockObject{
				{Type: slack.MarkdownType, Text: fmt.Sprintf("*Author:* %s", author)},
				{Type: slack.MarkdownType, Text: fmt.Sprintf("*Pipeline:* %s", info.PipelineName)},
			},
			nil,
		),
		slack.NewActionBlock(
			"",
			[]slack.BlockElement{
				&slack.ButtonBlockElement{
					Type:  slack.METButton,
					Style: slack.StylePrimary,
					URL:   info.BuildURL,
					Text:  &slack.TextBlockObject{Type: slack.PlainTextType, Text: "Go to build"},
				},
				&slack.ButtonBlockElement{
					Type: slack.METButton,
					URL:  "https://buildkite.com/organizations/sourcegraph/analytics/suites/sourcegraph-bazel?branch=main",
					Text: &slack.TextBlockObject{Type: slack.PlainTextType, Text: "View test analytics"},
				},
			}...,
		),

		&slack.DividerBlock{Type: slack.MBTDivider},

		slack.NewSectionBlock(
			&slack.TextBlockObject{
				Type: slack.MarkdownType,
				Text: `:books: *More information on flakes*
• <https://sourcegraph.com/docs/dev/background-information/ci#flakes|How to disable flaky tests>
• <https://github.com/sourcegraph/sourcegraph/issues/new/choose|Create a flaky test issue>
• <https://sourcegraph.com/docs/dev/how-to/testing#assessing-flaky-client-steps|Recognizing flaky client steps and how to fix them>

_Disable flakes on sight and save your fellow teammate some time!_`,
			},
			nil,
			nil,
		),
	}

	return blocks
}

func generateSlackHeader(info *BuildNotification) string {
	if len(info.Failed) == 0 && len(info.Fixed) > 0 {
		return fmt.Sprintf(":large_green_circle: Build %d fixed", info.BuildNumber)
	}
	header := fmt.Sprintf(":red_circle: Build %d failed", info.BuildNumber)
	switch info.ConsecutiveFailure {
	case 0, 1: // no suffix
	case 2:
		header += " (2nd failure)"
	case 3:
		header += " (:exclamation: 3rd failure)"
	default:
		header += fmt.Sprintf(" (:bangbang: %dth failure)", info.ConsecutiveFailure)
	}
	return header
}
