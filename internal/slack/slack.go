// Package slack is used to send notifications of an organization's activity
// to a given Slack webhook.
package slack

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"time"

	"github.com/sourcegraph/sourcegraph/internal/httpcli"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

// Client is capable of posting a message to a Slack webhook
type Client struct {
	WebhookURL string
}

// New creates a new Slack client
func New(webhookURL string) *Client {
	return &Client{WebhookURL: webhookURL}
}

// Payload is the wrapper for a Slack message, defined at:
// https://api.slack.com/docs/message-formatting
type Payload struct {
	Username    string        `json:"username,omitempty"`
	IconEmoji   string        `json:"icon_emoji,omitempty"`
	UnfurlLinks bool          `json:"unfurl_links,omitempty"`
	UnfurlMedia bool          `json:"unfurl_media,omitempty"`
	Text        string        `json:"text,omitempty"`
	Attachments []*Attachment `json:"attachments,omitempty"`
}

// Attachment is a Slack message attachment, defined at:
// https://api.slack.com/docs/message-attachments
type Attachment struct {
	AuthorIcon string   `json:"author_icon,omitempty"`
	AuthorLink string   `json:"author_link,omitempty"`
	AuthorName string   `json:"author_name,omitempty"`
	Color      string   `json:"color"`
	Fallback   string   `json:"fallback"`
	Fields     []*Field `json:"fields"`
	Footer     string   `json:"footer"`
	MarkdownIn []string `json:"mrkdwn_in"`
	ThumbURL   string   `json:"thumb_url"`
	Text       string   `json:"text,omitempty"`
	Timestamp  int64    `json:"ts"`
	Title      string   `json:"title"`
	TitleLink  string   `json:"title_link,omitempty"`
}

// Field is a single item within an attachment, defined at:
// https://api.slack.com/docs/message-attachments
type Field struct {
	Short bool   `json:"short"`
	Title string `json:"title"`
	Value string `json:"value"`
}

// Post sends payload to a Slack channel.
func (c *Client) Post(ctx context.Context, payload *Payload) error {
	if c.WebhookURL == "" {
		return nil
	}

	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return errors.Wrap(err, "slack: marshal json")
	}
	req, err := http.NewRequest("POST", c.WebhookURL, bytes.NewReader(payloadJSON))
	if err != nil {
		return errors.Wrap(err, "slack: create post request")
	}
	req.Header.Set("Content-Type", "application/json")

	timeoutCtx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()

	resp, err := httpcli.ExternalDoer.Do(req.WithContext(timeoutCtx))
	if err != nil {
		return errors.Wrap(err, "slack: http request")
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return err
		}
		return errors.Errorf("slack: %s failed with %d %s", payloadJSON, resp.StatusCode, string(body))
	}
	return nil
}
