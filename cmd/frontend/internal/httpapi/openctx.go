package httpapi

import (
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/sourcegraph/log"

	"github.com/sourcegraph/sourcegraph/cmd/frontend/internal/search"
	"github.com/sourcegraph/sourcegraph/internal/httpcli"
	"github.com/sourcegraph/sourcegraph/internal/trace"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

func serveOpenCtx(logger log.Logger) func(w http.ResponseWriter, r *http.Request) (err error) {
	return func(w http.ResponseWriter, r *http.Request) (err error) {
		// flagSet := featureflag.FromContext(r.Context())
		// if !flagSet.GetBoolOr("opencodegraph", false) {
		// 	return errors.New("OpenCodeGraph is not enabled (use the 'opencodegraph' feature flag)")
		// }

		if r.Method != "POST" {
			// The URL router should not have routed to this handler if method is not POST, but just
			// in case.
			return errors.New("method must be POST")
		}

		requestSource := search.GuessSource(r)
		r = r.WithContext(trace.WithRequestSource(r.Context(), requestSource))

		if r.Header.Get("Content-Encoding") == "gzip" {
			gzipReader, err := gzip.NewReader(r.Body)
			if err != nil {
				return errors.Wrap(err, "failed to decompress request body")
			}
			r.Body = gzipReader
			defer gzipReader.Close()
		}

		providerName := mux.Vars(r)["provider"]
		var methodReq struct {
			Method string `json:"method"`
		}
		reqBody, err := io.ReadAll(r.Body)
		if err != nil {
			return errors.Wrap(err, "failed to read request body")
		}

		if err := json.Unmarshal(reqBody, &methodReq); err != nil {
			return errors.Wrap(err, "failed to unmarshal request body")
		}

		providerFactory, ok := openCtxProviders[providerName]
		if !ok {
			return errors.Newf("unrecognized OpenCtx provider %q", providerName)
		}

		provider := providerFactory()

		type ResponseError struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
			Data    any    `json:"data"`
		}
		type response struct {
			Result any            `json:"result"`
			Error  *ResponseError `json:"error,omitempty"`
		}

		makeError := func(err error) *ResponseError {
			if err == nil {
				return nil
			}
			return &ResponseError{
				Code:    1,
				Message: err.Error(),
			}
		}

		switch methodReq.Method {
		case "meta":
			var req struct {
				Params MetaParams `json:"params"`
			}
			if err := json.Unmarshal(reqBody, &req); err != nil {
				return errors.Wrapf(err, "failed to decode request message")
			}

			meta, err := provider.Meta(r.Context(), r, req.Params)
			// if err != nil {
			// 	return errors.Wrapf(err, "failed to get meta")
			// }

			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(response{Result: meta, Error: makeError(err)})
			return nil
		case "items":
			var req struct {
				Params ItemsParams `json:"params"`
			}
			if err := json.Unmarshal(reqBody, &req); err != nil {
				return errors.Wrapf(err, "failed to decode request message")
			}

			items, err := provider.Items(r.Context(), r, req.Params)
			// if err != nil {
			// 	return errors.Wrapf(err, "failed to get items")
			// }

			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(response{Result: items, Error: makeError(err)})
			return nil
		case "annotations":
			var req struct {
				Params AnnotationsParams `json:"params"`
			}
			if err := json.Unmarshal(reqBody, &req); err != nil {
				return errors.Wrapf(err, "failed to decode request message")
			}

			annotations, err := provider.Annotations(r.Context(), r, req.Params.URI, []byte(req.Params.Content))
			// if err != nil {
			// 	return errors.Wrapf(err, "failed to get annotations")
			// }

			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(response{Result: annotations, Error: makeError(err)})
			return nil
		case "mentions":
			var req struct {
				Params MentionsParams `json:"params"`
			}
			if err := json.Unmarshal(reqBody, &req); err != nil {
				return errors.Wrapf(err, "failed to decode request message")
			}

			mentions, err := provider.Mentions(r.Context(), r, req.Params)
			// if err != nil {
			// 	return errors.Wrapf(err, "failed to get mentions")
			// }

			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(response{Result: mentions, Error: makeError(err)})
			return nil
		default:
			return errors.Newf("unrecognized OpenCtx request method %q", methodReq.Method)
		}
	}
}

var openCtxProviders = map[string]func() OpenCtxProvider{
	"sentry": func() OpenCtxProvider {
		return &sentryOpenCtxProvider{
			apiToken: os.Getenv("OPENCTX_SENTRY_API_TOKEN"),
		}
	},
}

type AnnotationsParams struct {
	URI     string `json:"uri"`
	Content string `json:"content"`
}

type MetaParams struct {
	// empty for now
}

type ItemsParams struct {
	Message string   `json:"message"`
	Mention *Mention `json:"mention,omitempty"`
}

type MetaResult struct {
	/**
	 * Selects the scope in which this provider should be called.
	 *
	 * If one or more selectors are given, all must be satisfied for the
	 * provider to be called. If undefined, the provider is called on all resources.
	 * If empty, the provider is never invoked.
	 */
	Selector []Selector   `json:"selector,omitempty"`
	Name     string       `json:"name"`
	Features MetaFeatures `json:"features"`
}

type MetaFeatures struct {
	Mentions bool `json:"mentions"`
}

/**
 * Defines a scope in which a provider is called.
 *
 * To satisfy a selector, all of the selector's conditions must be met. For
 * example, if both `path` and `content` are specified, the resource must satisfy
 * both conditions.
 */
type Selector struct {
	/**
	 * A glob that must match the resource's hostname and path.
	 */
	Path string `json:"path"`

	/**
	 * A literal string that must be present in the resource's content for the
	 * provider to be called.
	 */
	ContentContains string `json:"contentContains"`
}

type MentionsParams struct {
	Query string `json:"query"`
}

type OpenCtxProvider interface {
	Meta(ctx context.Context, req *http.Request, params MetaParams) (MetaResult, error)
	Items(ctx context.Context, req *http.Request, params ItemsParams) ([]Item, error)
	Annotations(ctx context.Context, req *http.Request, uri string, content []byte) ([]Annotation, error)
	Mentions(ctx context.Context, req *http.Request, params MentionsParams) ([]Mention, error)
}

// A mention contains presentation information relevant to a resource.
type Mention struct {
	// A descriptive title.
	Title string `json:"title"`
	// A URI for the mention item.
	URI  string                     `json:"uri"`
	Data map[string]json.RawMessage `json:"data"`
}

/**
* An annotation attaches an Item to a range in a document.
 */
type Annotation struct {
	/** The URI of the document. */
	URI string `json:"uri"`

	/**
	 * The range in the resource that this item applies to. If not set, the item applies to the entire resource.
	 */
	Range AnnotationRange `json:"range,omitempty"`

	/** The item containing the content to annotate at the range. */
	Item Item `json:"item"`
}

type AnnotationRange struct {
	Start Position `json:"start"`
	End   Position `json:"end"`
}

/**
 * An item describes information relevant to a resource (or a range within a resource).
 */
type Item struct {
	/**
	 * A descriptive title of the item.
	 */
	Title string `json:"title"`

	/**
	 * An external URL with more information about the item.
	 */
	URL string `json:"url,omitempty"`

	/**
	 * The human user interface of the item, with information for human consumption.
	 */
	UI *UserInterface `json:"ui,omitempty"`

	/**
	 * Information from the item intended for consumption by AI, not humans.
	 */
	AI *AssistantInfo `json:"ai,omitempty"`
}

/**
 * The human user interface of the item, with information for human consumption.
 */
type UserInterface struct {
	/**
	 * Additional information for the human, shown in a tooltip-like widget when they interact with the item.
	 */
	Hover UserInterfaceHover `json:"hover,omitempty"`
}

type UserInterfaceHover struct {
	Markdown string `json:"markdown,omitempty"`
	Text     string `json:"text,omitempty"`
}

/**
 * Information from the item intended for consumption by AI, not humans.
 */
type AssistantInfo struct {
	/**
	 * Text content for AI to consume.
	 */
	Content string `json:"content,omitempty"`
}

type Position struct {
	/** Line number (0-indexed). */
	Line uint `json:"line"`

	/** Character offset on line (0-indexed). */
	Character uint `json:"character"`
}

type sentryOpenCtxProvider struct {
	apiToken string
}

func (p *sentryOpenCtxProvider) Meta(ctx context.Context, req *http.Request, params MetaParams) (MetaResult, error) {
	return MetaResult{
		// empty since we don't provide any annotations.
		Selector: []Selector{},
		Name:     "Sentry issues",
		Features: MetaFeatures{
			Mentions: true,
		},
	}, nil
}

func (p *sentryOpenCtxProvider) Items(ctx context.Context, _ *http.Request, params ItemsParams) ([]Item, error) {
	if params.Mention == nil || params.Mention.Data == nil {
		return []Item{}, nil
	}

	issueRaw, ok := params.Mention.Data["issue"]
	if !ok {
		return []Item{}, nil
	}

	var issue Issue
	if err := json.Unmarshal(issueRaw, &issue); err != nil {
		return nil, errors.Wrap(err, "unmarshaling issue")
	}

	return []Item{
		{
			Title: "Sentry issue",
			AI:    &AssistantInfo{Content: aiContentForIssue(&issue)},
			UI:    &UserInterface{Hover: UserInterfaceHover{Text: issue.Title}},
			URL:   issue.Permalink,
		},
	}, nil
}

func aiContentForIssue(issue *Issue) string {
	return fmt.Sprintf("**%s**\n\n%s\n\n%s", issue.Title, issue.Culprit, issue.Permalink)
}

func (p *sentryOpenCtxProvider) Annotations(ctx context.Context, req *http.Request, uri string, content []byte) ([]Annotation, error) {
	// Not supported.
	return nil, nil
}

func (p *sentryOpenCtxProvider) Mentions(ctx context.Context, req *http.Request, params MentionsParams) ([]Mention, error) {
	// Parse URL:
	// Example format https://sourcegraph.sentry.io/issues/4515694084/?project=6583153&query=is%3Aunresolved+issue.priority%3A%5Bhigh%2C+medium%5D&referrer=issue-stream&statsPeriod=14d&stream_index=0
	u, err := url.Parse(params.Query)
	if err != nil {
		return []Mention{}, nil
	}

	if !strings.Contains(u.Hostname(), "sentry.io") {
		return []Mention{}, nil
	}

	organizationId := strings.Split(u.Hostname(), ".")[0]
	issueId := strings.Split(u.Path, "/")[2]

	if organizationId == "" || issueId == "" {
		return []Mention{}, nil
	}

	ctx, cancel := context.WithDeadline(ctx, time.Now().Add(3*time.Second))
	defer cancel()

	issue, err := fetchSentryIssueDetails(ctx, p.apiToken, organizationId, issueId)
	if err != nil {
		return nil, errors.Wrap(err, "could not fetch issue details")
	}

	return []Mention{
		{
			Title: "Sentry issue: " + issue.Title,
			URI:   issue.Permalink,
			Data: map[string]json.RawMessage{
				"content": mustMarshalToRaw(issue.Title),
				"issue":   mustMarshalToRaw(issue),
			},
		},
	}, nil
}

func mustMarshalToRaw(a any) json.RawMessage {
	b, err := json.Marshal(a)
	if err != nil {
		panic(err)
	}
	return json.RawMessage(b)
}

type Issue struct {
	Activity []struct {
		Data        map[string]interface{} `json:"data"`
		DateCreated string                 `json:"dateCreated"`
		ID          string                 `json:"id"`
		Type        string                 `json:"type"`
		User        map[string]interface{} `json:"user"`
	} `json:"activity"`
	Annotations  []string               `json:"annotations"`
	AssignedTo   map[string]interface{} `json:"assignedTo"`
	Count        string                 `json:"count"`
	Culprit      string                 `json:"culprit"`
	FirstRelease struct {
		Authors      []string               `json:"authors"`
		CommitCount  int                    `json:"commitCount"`
		Data         map[string]interface{} `json:"data"`
		DateCreated  string                 `json:"dateCreated"`
		DateReleased string                 `json:"dateReleased"`
		DeployCount  int                    `json:"deployCount"`
		FirstEvent   string                 `json:"firstEvent"`
		LastCommit   string                 `json:"lastCommit"`
		LastDeploy   string                 `json:"lastDeploy"`
		LastEvent    string                 `json:"lastEvent"`
		NewGroups    int                    `json:"newGroups"`
		Owner        string                 `json:"owner"`
		Projects     []struct {
			Name string `json:"name"`
			Slug string `json:"slug"`
		} `json:"projects"`
		Ref          string `json:"ref"`
		ShortVersion string `json:"shortVersion"`
		URL          string `json:"url"`
		Version      string `json:"version"`
	} `json:"firstRelease"`
	FirstSeen    string                 `json:"firstSeen"`
	HasSeen      bool                   `json:"hasSeen"`
	ID           string                 `json:"id"`
	IsBookmarked bool                   `json:"isBookmarked"`
	IsPublic     bool                   `json:"isPublic"`
	IsSubscribed bool                   `json:"isSubscribed"`
	LastRelease  map[string]interface{} `json:"lastRelease"`
	LastSeen     string                 `json:"lastSeen"`
	Level        string                 `json:"level"`
	Logger       string                 `json:"logger"`
	Metadata     struct {
		Filename string `json:"filename"`
		Type     string `json:"type"`
		Value    string `json:"value"`
	} `json:"metadata"`
	NumComments    int                      `json:"numComments"`
	Participants   []map[string]interface{} `json:"participants"`
	Permalink      string                   `json:"permalink"`
	PluginActions  [][]string               `json:"pluginActions"`
	PluginContexts []string                 `json:"pluginContexts"`
	PluginIssues   []map[string]interface{} `json:"pluginIssues"`
	Project        struct {
		ID   string `json:"id"`
		Name string `json:"name"`
		Slug string `json:"slug"`
	} `json:"project"`
	SeenBy  []map[string]interface{} `json:"seenBy"`
	ShareID string                   `json:"shareId"`
	ShortID string                   `json:"shortId"`
	Stats   struct {
		TwentyFourHours [][]int `json:"24h"`
		ThirtyDays      [][]int `json:"30d"`
	} `json:"stats"`
	Status              string                   `json:"status"`
	StatusDetails       map[string]interface{}   `json:"statusDetails"`
	SubscriptionDetails map[string]interface{}   `json:"subscriptionDetails"`
	Tags                []map[string]interface{} `json:"tags"`
	Title               string                   `json:"title"`
	Type                string                   `json:"type"`
	UserCount           int                      `json:"userCount"`
	UserReportCount     int                      `json:"userReportCount"`
}

func fetchSentryIssueDetails(ctx context.Context, token, orgID, issueID string) (*Issue, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", "https://sentry.io/api/0/organizations/"+orgID+"/issues/"+issueID+"/", nil)
	if err != nil {
		return nil, errors.Wrap(err, "could not create request")
	}
	req.Header.Add("Authorization", "Bearer "+token)

	resp, err := httpcli.UncachedExternalDoer.Do(req)
	if err != nil {
		return nil, errors.Wrap(err, "could not fetch issue")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, errors.New("unexpected status code: " + resp.Status)
	}

	var i Issue
	return &i, errors.Wrap(json.NewDecoder(resp.Body).Decode(&i), "failed to unmarshal issue")
}
