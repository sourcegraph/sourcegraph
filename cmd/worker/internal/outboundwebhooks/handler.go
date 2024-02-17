package outboundwebhooks

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"

	"github.com/sourcegraph/conc/pool"
	"github.com/sourcegraph/log"

	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/encryption"
	"github.com/sourcegraph/sourcegraph/internal/httpcli"
	"github.com/sourcegraph/sourcegraph/internal/types"
	"github.com/sourcegraph/sourcegraph/internal/webhooks/outbound"
	"github.com/sourcegraph/sourcegraph/internal/workerutil"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

type handler struct {
	client   httpcli.Doer
	store    database.OutboundWebhookStore
	logStore database.OutboundWebhookLogStore
}

var _ workerutil.Handler[*types.OutboundWebhookJob] = &handler{}

func (h *handler) Handle(ctx context.Context, logger log.Logger, job *types.OutboundWebhookJob) error {
	logger = logger.With(
		log.Int64("job.id", job.ID),
		log.String("job.event_type", job.EventType),
		log.Stringp("job.scope", job.Scope),
	)

	webhooks, err := h.store.List(ctx, database.OutboundWebhookListOpts{
		OutboundWebhookCountOpts: database.OutboundWebhookCountOpts{
			EventTypes: []database.FilterEventType{{
				EventType: job.EventType,
				Scope:     job.Scope,
			}},
		},
	})
	if err != nil {
		logger.Error("error retrieving outbound webhooks", log.Error(err))
		return errors.Wrap(err, "retrieving outbound webhooks")
	}

	// Since sending HTTP requests is (generally) cheap, and we've already done
	// the relatively expensive parts of constructing the payload and retrieving
	// the matching hooks, we're going to just fan these out with a high
	// concurrency limit.
	p := pool.New().WithContext(ctx).WithMaxGoroutines(100)
	for _, webhook := range webhooks {
		p.Go(h.buildWebhookSender(
			logger.With(log.Int64("webhook.id", webhook.ID)),
			job, webhook,
		))
	}

	// Errors will have been logged individually, so we can just return the
	// error back out of the handler.
	return p.Wait()
}

func (h *handler) buildWebhookSender(
	logger log.Logger, job *types.OutboundWebhookJob,
	webhook *types.OutboundWebhook,
) func(context.Context) error {
	return func(ctx context.Context) error {
		return h.sendWebhook(ctx, logger, job, webhook)
	}
}

func (h *handler) sendWebhook(
	ctx context.Context, logger log.Logger,
	job *types.OutboundWebhookJob, webhook *types.OutboundWebhook,
) error {
	// This function is a bit of a god function, but there isn't an obvious way
	// to break it down — in classic Go style, much of its weight is really just
	// repetitive error handling.

	logger.Debug("preparing to send webhook payload")

	// First, we need to decrypt the values we need that may be encrypted.
	url, err := webhook.URL.Decrypt(ctx)
	if err != nil {
		logger.Error("cannot decrypt webhook URL", log.Error(err))
		return errors.Wrap(err, "decrypting webhook URL")
	}

	err = outbound.CheckURL(url)
	if err != nil {
		logger.Error("webhook URL is not allowed", log.Error(err))
		return errors.Wrap(err, "checking webhook URL")
	}

	secret, err := webhook.Secret.Decrypt(ctx)
	if err != nil {
		logger.Error("cannot decrypt webhook secret", log.Error(err))
		return errors.Wrap(err, "decrypting webhook secret")
	}

	payload, err := job.Payload.Decrypt(ctx)
	if err != nil {
		logger.Error("cannot decrypt payload", log.Error(err))
		return errors.Wrap(err, "decrypting payload")
	}

	// Second, we need to generate a signature based on the shared secret and
	// the payload contents.
	payloadReader := bytes.NewReader([]byte(payload))
	sig, err := calculateSignature(secret, payloadReader)
	if err != nil {
		logger.Error("error signing payload", log.Error(err))
		return errors.Wrap(err, "calculating payload signature")
	}
	payloadReader.Seek(0, io.SeekStart)

	// Third, we build the HTTP request.
	req, err := http.NewRequestWithContext(ctx, "POST", url, payloadReader)
	if err != nil {
		logger.Error("cannot build webhook request", log.Error(err))
		return errors.Wrap(err, "building request")
	}

	req.Header.Add("Content-Type", "application/json; charset=utf-8")
	req.Header.Add("X-Sourcegraph-Webhook-Event-Type", job.EventType)
	req.Header.Add("X-Sourcegraph-Webhook-Signature", sig)

	// Fourth, we set up the outbound webhook logging, since at this point we
	// now know we'll send the request.
	webhookLog := &types.OutboundWebhookLog{
		JobID:             job.ID,
		OutboundWebhookID: webhook.ID,
		Request: types.NewUnencryptedWebhookLogMessage(types.WebhookLogMessage{
			Header: req.Header,
			Body:   []byte(payload),
			Method: req.Method,
			URL:    url,
		}),
		Response: types.NewUnencryptedWebhookLogMessage(types.WebhookLogMessage{}),
		Error:    encryption.NewUnencrypted(""),
	}
	defer func() {
		if err := h.logStore.Create(ctx, webhookLog); err != nil {
			// We don't want to return an error from the overall send function
			// if we can't write a log entry, so we'll log at a level that will
			// hopefully gather some attention and then swallow the error.
			logger.Warn("error writing outbound webhook log", log.Error(err))
		}
	}()

	// Fifth, we actually send the request.
	resp, err := h.client.Do(req)
	if err != nil {
		logger.Info("error sending webhook", log.Error(err))
		webhookLog.Error = encryption.NewUnencrypted(err.Error())
		return errors.Wrap(err, "sending webhook")
	}

	// Sixth, we process the response for logging purposes.
	defer resp.Body.Close()
	webhookLog.StatusCode = resp.StatusCode

	response, err := io.ReadAll(resp.Body)
	if err != nil {
		logger.Error("cannot read response body", log.Error(err))
		return errors.Wrap(err, "reading response body")
	}
	webhookLog.Response = types.NewUnencryptedWebhookLogMessage(types.WebhookLogMessage{
		Header: resp.Header,
		Body:   response,
		Method: req.Method,
		URL:    url,
	})

	if resp.StatusCode >= http.StatusBadRequest {
		logger.Info("got unexpected status code from webhook", log.Int("status_code", resp.StatusCode))
		return errors.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	logger.Debug("webhook sent successfully")
	return nil
}

func calculateSignature(secret string, payload io.Reader) (string, error) {
	mac := hmac.New(sha256.New, []byte(secret))
	if _, err := io.Copy(mac, payload); err != nil {
		return "", err
	}

	return hex.EncodeToString(mac.Sum(nil)), nil
}
