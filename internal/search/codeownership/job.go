package codeownership

import (
	"context"
	"strings"

	otlog "github.com/opentracing/opentracing-go/log"

	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/search"
	"github.com/sourcegraph/sourcegraph/internal/search/job"
	"github.com/sourcegraph/sourcegraph/internal/search/result"
	"github.com/sourcegraph/sourcegraph/internal/search/streaming"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

func New(child job.Job, includeOwners, excludeOwners []string, db database.DB) job.Job {
	return &codeownershipJob{
		child:         child,
		includeOwners: includeOwners,
		excludeOwners: excludeOwners,
		db:            db,
	}
}

type codeownershipJob struct {
	child         job.Job
	includeOwners []string
	excludeOwners []string
	db            database.DB
}

func (s *codeownershipJob) Run(ctx context.Context, clients job.RuntimeClients, stream streaming.Sender) (alert *search.Alert, err error) {
	_, ctx, stream, finish := job.StartSpan(ctx, stream, s)
	defer func() { finish(alert, err) }()

	var errs error

	filteredStream := streaming.StreamFunc(func(event streaming.SearchEvent) {
		event.Results, _ = applyCodeOwnershipFiltering(ctx, s.includeOwners, s.excludeOwners, event.Results)
		stream.Send(event)
	})

	alert, err = s.child.Run(ctx, clients, filteredStream)
	if err != nil {
		errs = errors.Append(errs, err)
	}
	return alert, errs
}

func (s *codeownershipJob) Name() string {
	return "CodeOwnershipFilterJob"
}

func (s *codeownershipJob) Fields(v job.Verbosity) (res []otlog.Field) {
	switch v {
	case job.VerbosityMax:
		fallthrough
	case job.VerbosityBasic:
		res = append(res,
			otlog.String("includeOwners", strings.Join(s.includeOwners, ",")),
			otlog.String("excludeOwners", strings.Join(s.excludeOwners, ",")),
		)
	}
	return res
}

func (s *codeownershipJob) Children() []job.Describer {
	return []job.Describer{s.child}
}

func (s *codeownershipJob) MapChildren(fn job.MapFunc) job.Job {
	cp := *s
	cp.child = job.Map(s.child, fn)
	return &cp
}

func applyCodeOwnershipFiltering(ctx context.Context, includeOwners, excludeOwners []string, matches []result.Match) ([]result.Match, error) {
	filtered := matches[:0]

	// We currently don't have a way to access file ownership information, so no
	// file currently has any owner. A search to include any owner will
	// therefore return no results.
	if len(includeOwners) == 0 {
		filtered = matches
	}

	return filtered, nil
}
