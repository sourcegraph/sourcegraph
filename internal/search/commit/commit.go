package commit

import (
	"context"
	"strings"
	"time"

	"github.com/grafana/regexp"
	"github.com/sourcegraph/conc/pool"
	"go.opentelemetry.io/otel/attribute"

	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/errcode"
	"github.com/sourcegraph/sourcegraph/internal/gitserver"
	"github.com/sourcegraph/sourcegraph/internal/gitserver/gitdomain"
	"github.com/sourcegraph/sourcegraph/internal/gitserver/protocol"
	gitprotocol "github.com/sourcegraph/sourcegraph/internal/gitserver/protocol"
	"github.com/sourcegraph/sourcegraph/internal/search"
	"github.com/sourcegraph/sourcegraph/internal/search/job"
	"github.com/sourcegraph/sourcegraph/internal/search/query"
	"github.com/sourcegraph/sourcegraph/internal/search/result"
	"github.com/sourcegraph/sourcegraph/internal/search/streaming"
	"github.com/sourcegraph/sourcegraph/internal/types"
)

type SearchJob struct {
	Query                gitprotocol.Node
	Repos                []*search.RepositoryRevisions
	Diff                 bool
	Limit                int
	IncludeModifiedFiles bool
	Concurrency          int

	// CodeMonitorSearchWrapper, if set, will wrap the commit search with extra logic specific to code monitors.
	CodeMonitorSearchWrapper CodeMonitorHook `json:"-"`
}

type DoSearchFunc func(*gitprotocol.SearchRequest) error
type CodeMonitorHook func(context.Context, database.DB, GitserverClient, *gitprotocol.SearchRequest, api.RepoID, DoSearchFunc) error

type GitserverClient interface {
	Search(_ context.Context, _ *protocol.SearchRequest, onMatches func([]protocol.CommitMatch)) (limitHit bool, _ error)
	ResolveRevision(context.Context, api.RepoName, string, gitserver.ResolveRevisionOptions) (api.CommitID, error)
}

func (j *SearchJob) Run(ctx context.Context, clients job.RuntimeClients, stream streaming.Sender) (alert *search.Alert, err error) {
	_, ctx, stream, finish := job.StartSpan(ctx, stream, j)
	defer func() { finish(alert, err) }()

	if err := j.ExpandUsernames(ctx, clients.DB); err != nil {
		return nil, err
	}

	searchRepoRev := func(ctx context.Context, repoRev *search.RepositoryRevisions) error {
		// Skip the repo if no revisions were resolved for it
		if len(repoRev.Revs) == 0 {
			return nil
		}

		args := &protocol.SearchRequest{
			Repo:                 repoRev.Repo.Name,
			Revisions:            repoRev.Revs,
			Query:                j.Query,
			IncludeDiff:          j.Diff,
			Limit:                j.Limit,
			IncludeModifiedFiles: j.IncludeModifiedFiles,
		}

		onMatches := func(in []protocol.CommitMatch) {
			res := make([]result.Match, 0, len(in))
			for _, protocolMatch := range in {
				res = append(res, protocolMatchToCommitMatch(repoRev.Repo, j.Diff, protocolMatch))
			}
			stream.Send(streaming.SearchEvent{
				Results: res,
			})
		}

		doSearch := func(args *gitprotocol.SearchRequest) error {
			limitHit, err := clients.Gitserver.Search(ctx, args, onMatches)
			statusMap, limitHit, err := search.HandleRepoSearchResult(repoRev.Repo.ID, repoRev.Revs, limitHit, false, err)
			stream.Send(streaming.SearchEvent{
				Stats: streaming.Stats{
					IsLimitHit: limitHit,
					Status:     statusMap,
				},
			})
			return err
		}

		if j.CodeMonitorSearchWrapper != nil {
			return j.CodeMonitorSearchWrapper(ctx, clients.DB, clients.Gitserver, args, repoRev.Repo.ID, doSearch)
		}
		return doSearch(args)
	}

	p := pool.New().WithContext(ctx).WithMaxGoroutines(4).WithFirstError()

	for _, repoRev := range j.Repos {
		repoRev := repoRev
		p.Go(func(ctx context.Context) error {
			return searchRepoRev(ctx, repoRev)
		})
	}

	return nil, p.Wait()
}

func (j *SearchJob) Name() string {
	if j.Diff {
		return "DiffSearchJob"
	}
	return "CommitSearchJob"
}

func (j *SearchJob) Attributes(v job.Verbosity) (res []attribute.KeyValue) {
	switch v {
	case job.VerbosityMax:
		res = append(res,
			attribute.Bool("includeModifiedFiles", j.IncludeModifiedFiles),
		)
		fallthrough
	case job.VerbosityBasic:
		res = append(res,
			attribute.Stringer("query", j.Query),
			attribute.Bool("diff", j.Diff),
			attribute.Int("limit", j.Limit),
		)
	}
	return res
}

func (j *SearchJob) Children() []job.Describer       { return nil }
func (j *SearchJob) MapChildren(job.MapFunc) job.Job { return j }

func (j *SearchJob) ExpandUsernames(ctx context.Context, db database.DB) (err error) {
	protocol.ReduceWith(j.Query, func(n protocol.Node) protocol.Node {
		if err != nil {
			return n
		}

		var expr *string
		switch v := n.(type) {
		case *protocol.AuthorMatches:
			expr = &v.Expr
		case *protocol.CommitterMatches:
			expr = &v.Expr
		default:
			return n
		}

		var expanded []string
		expanded, err = expandUsernamesToEmails(ctx, db, []string{*expr})
		if err != nil {
			return n
		}

		*expr = "(?:" + strings.Join(expanded, ")|(?:") + ")"
		return n
	})
	return err
}

// expandUsernamesToEmails expands references to usernames to mention all possible (known and
// verified) email addresses for the user.
//
// For example, given a list ["foo", "@alice"] where the user "alice" has 2 email addresses
// "alice@example.com" and "alice@example.org", it would return ["foo", "alice@example\\.com",
// "alice@example\\.org"].
func expandUsernamesToEmails(ctx context.Context, db database.DB, values []string) (expandedValues []string, err error) {
	expandOne := func(ctx context.Context, value string) ([]string, error) {
		if isPossibleUsernameReference := strings.HasPrefix(value, "@"); !isPossibleUsernameReference {
			return nil, nil
		}

		user, err := db.Users().GetByUsername(ctx, strings.TrimPrefix(value, "@"))
		if errcode.IsNotFound(err) {
			return nil, nil
		} else if err != nil {
			return nil, err
		}
		emails, err := db.UserEmails().ListByUser(ctx, database.UserEmailsListOptions{
			UserID: user.ID,
		})
		if err != nil {
			return nil, err
		}
		values := make([]string, 0, len(emails))
		for _, email := range emails {
			if email.VerifiedAt != nil {
				values = append(values, regexp.QuoteMeta(email.Email))
			}
		}
		return values, nil
	}

	expandedValues = make([]string, 0, len(values))
	for _, v := range values {
		x, err := expandOne(ctx, v)
		if err != nil {
			return nil, err
		}
		if x == nil {
			expandedValues = append(expandedValues, v) // not a username or couldn't expand
		} else {
			expandedValues = append(expandedValues, x...)
		}
	}
	return expandedValues, nil
}

func QueryToGitQuery(b query.Basic, diff bool) gitprotocol.Node {
	caseSensitive := b.IsCaseSensitive()

	res := make([]gitprotocol.Node, 0, len(b.Parameters)+2)

	// Convert parameters to nodes
	for _, parameter := range b.Parameters {
		if parameter.Annotation.Labels.IsSet(query.IsPredicate) {
			continue
		}
		newPred := queryParameterToPredicate(parameter, caseSensitive, diff)
		if newPred != nil {
			res = append(res, newPred)
		}
	}

	// Convert pattern to nodes
	newPred := queryPatternToPredicate(b.Pattern, caseSensitive, diff)
	if newPred != nil {
		res = append(res, newPred)
	}

	return gitprotocol.Reduce(gitprotocol.NewAnd(res...))
}

func queryPatternToPredicate(node query.Node, caseSensitive, diff bool) gitprotocol.Node {
	switch v := node.(type) {
	case query.Operator:
		return patternOperatorToPredicate(v, caseSensitive, diff)
	case query.Pattern:
		return patternAtomToPredicate(v, caseSensitive, diff)
	default:
		// Invariant: the node passed to queryPatternToPredicate should only contain pattern nodes
		return nil
	}
}

func patternOperatorToPredicate(op query.Operator, caseSensitive, diff bool) gitprotocol.Node {
	switch op.Kind {
	case query.And:
		return gitprotocol.NewAnd(patternNodesToPredicates(op.Operands, caseSensitive, diff)...)
	case query.Or:
		return gitprotocol.NewOr(patternNodesToPredicates(op.Operands, caseSensitive, diff)...)
	default:
		return nil
	}
}

func patternNodesToPredicates(nodes []query.Node, caseSensitive, diff bool) []gitprotocol.Node {
	res := make([]gitprotocol.Node, 0, len(nodes))
	for _, node := range nodes {
		newPred := queryPatternToPredicate(node, caseSensitive, diff)
		if newPred != nil {
			res = append(res, newPred)
		}
	}
	return res
}

func patternAtomToPredicate(pattern query.Pattern, caseSensitive, diff bool) gitprotocol.Node {
	patString := pattern.RegExpPattern()

	var newPred gitprotocol.Node
	if diff {
		newPred = &gitprotocol.DiffMatches{Expr: patString, IgnoreCase: !caseSensitive}
	} else {
		newPred = &gitprotocol.MessageMatches{Expr: patString, IgnoreCase: !caseSensitive}
	}

	if pattern.Negated {
		return gitprotocol.NewNot(newPred)
	}
	return newPred
}

func queryParameterToPredicate(parameter query.Parameter, caseSensitive, diff bool) gitprotocol.Node {
	var newPred gitprotocol.Node
	switch parameter.Field {
	case query.FieldAuthor:
		// TODO(@camdencheek) look up emails (issue #25180)
		newPred = &gitprotocol.AuthorMatches{Expr: parameter.Value, IgnoreCase: !caseSensitive}
	case query.FieldCommitter:
		newPred = &gitprotocol.CommitterMatches{Expr: parameter.Value, IgnoreCase: !caseSensitive}
	case query.FieldBefore:
		t, _ := query.ParseGitDate(parameter.Value, time.Now) // field already validated
		newPred = &gitprotocol.CommitBefore{Time: t}
	case query.FieldAfter:
		t, _ := query.ParseGitDate(parameter.Value, time.Now) // field already validated
		newPred = &gitprotocol.CommitAfter{Time: t}
	case query.FieldMessage:
		newPred = &gitprotocol.MessageMatches{Expr: parameter.Value, IgnoreCase: !caseSensitive}
	case query.FieldContent:
		if diff {
			newPred = &gitprotocol.DiffMatches{Expr: parameter.Value, IgnoreCase: !caseSensitive}
		} else {
			newPred = &gitprotocol.MessageMatches{Expr: parameter.Value, IgnoreCase: !caseSensitive}
		}
	case query.FieldFile:
		newPred = &gitprotocol.DiffModifiesFile{Expr: parameter.Value, IgnoreCase: !caseSensitive}
	case query.FieldLang:
		newPred = &gitprotocol.DiffModifiesFile{Expr: query.LangToFileRegexp(parameter.Value), IgnoreCase: true}
	}

	if parameter.Negated && newPred != nil {
		return gitprotocol.NewNot(newPred)
	}
	return newPred
}

func protocolMatchToCommitMatch(repo types.MinimalRepo, diff bool, in protocol.CommitMatch) *result.CommitMatch {
	var diffPreview, messagePreview *result.MatchedString
	var structuredDiff []result.DiffFile
	if diff {
		diffPreview = &in.Diff
		structuredDiff, _ = result.ParseDiffString(in.Diff.Content)
	} else {
		messagePreview = &in.Message
	}

	return &result.CommitMatch{
		Commit: gitdomain.Commit{
			ID: in.Oid,
			Author: gitdomain.Signature{
				Name:  in.Author.Name,
				Email: in.Author.Email,
				Date:  in.Author.Date,
			},
			Committer: &gitdomain.Signature{
				Name:  in.Committer.Name,
				Email: in.Committer.Email,
				Date:  in.Committer.Date,
			},
			Message: gitdomain.Message(in.Message.Content),
			Parents: in.Parents,
		},
		Repo:           repo,
		DiffPreview:    diffPreview,
		Diff:           structuredDiff,
		MessagePreview: messagePreview,
		ModifiedFiles:  in.ModifiedFiles,
	}
}
