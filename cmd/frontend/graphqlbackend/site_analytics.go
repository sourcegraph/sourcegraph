package graphqlbackend

import (
	"context"

	"github.com/sourcegraph/sourcegraph/internal/adminanalytics"
	"github.com/sourcegraph/sourcegraph/internal/auth"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/featureflag"
)

type siteAnalyticsResolver struct {
	db    database.DB
	cache bool
}

/* Analytics root resolver */
func (r *siteResolver) Analytics(ctx context.Context) (*siteAnalyticsResolver, error) {
	if err := auth.CheckCurrentUserIsSiteAdmin(ctx, r.db); err != nil {
		return nil, err
	}

	cache := !featureflag.FromContext(ctx).GetBoolOr("admin-analytics-cache-disabled", false)

	return &siteAnalyticsResolver{r.db, cache}, nil
}

/* Search */

func (r *siteAnalyticsResolver) Search(ctx context.Context, args *struct {
	DateRange *string
	Grouping  *string
}) *adminanalytics.Search {
	return &adminanalytics.Search{Ctx: ctx, DateRange: *args.DateRange, Grouping: *args.Grouping, DB: r.db, Cache: r.cache}
}

/* Notebooks */

func (r *siteAnalyticsResolver) Notebooks(ctx context.Context, args *struct {
	DateRange *string
	Grouping  *string
}) *adminanalytics.Notebooks {
	return &adminanalytics.Notebooks{Ctx: ctx, DateRange: *args.DateRange, Grouping: *args.Grouping, DB: r.db, Cache: r.cache}
}

/* Users */

func (r *siteAnalyticsResolver) Users(ctx context.Context, args *struct {
	DateRange *string
	Grouping  *string
}) (*adminanalytics.Users, error) {
	return &adminanalytics.Users{Ctx: ctx, DateRange: *args.DateRange, Grouping: *args.Grouping, DB: r.db, Cache: r.cache}, nil
}

/* Code-intel */

func (r *siteAnalyticsResolver) CodeIntel(ctx context.Context, args *struct {
	DateRange *string
	Grouping  *string
}) *adminanalytics.CodeIntel {
	return &adminanalytics.CodeIntel{Ctx: ctx, DateRange: *args.DateRange, Grouping: *args.Grouping, DB: r.db, Cache: r.cache}
}

/* Code-intel by language */

func (r *siteAnalyticsResolver) CodeIntelByLanguage(ctx context.Context, args *struct {
	DateRange *string
}) ([]*adminanalytics.CodeIntelByLanguage, error) {
	return adminanalytics.GetCodeIntelByLanguage(ctx, r.db, r.cache, *args.DateRange)
}

/* Code-intel by language */

func (r *siteAnalyticsResolver) CodeIntelTopRepositories(ctx context.Context, args *struct {
	DateRange *string
}) ([]*adminanalytics.CodeIntelTopRepositories, error) {
	return adminanalytics.GetCodeIntelTopRepositories(ctx, r.db, r.cache, *args.DateRange)
}

/* Repos */

func (r *siteAnalyticsResolver) Repos(ctx context.Context) (*adminanalytics.ReposSummary, error) {
	repos := adminanalytics.Repos{DB: r.db, Cache: r.cache}

	return repos.Summary(ctx)
}

/* Batch changes */

func (r *siteAnalyticsResolver) BatchChanges(ctx context.Context, args *struct {
	DateRange *string
	Grouping  *string
}) *adminanalytics.BatchChanges {
	return &adminanalytics.BatchChanges{Ctx: ctx, DateRange: *args.DateRange, Grouping: *args.Grouping, DB: r.db, Cache: r.cache}
}

/* Extensions */

func (r *siteAnalyticsResolver) Extensions(ctx context.Context, args *struct {
	DateRange *string
	Grouping  *string
}) *adminanalytics.Extensions {
	return &adminanalytics.Extensions{Ctx: ctx, DateRange: *args.DateRange, Grouping: *args.Grouping, DB: r.db, Cache: r.cache}
}

/* Insights */

func (r *siteAnalyticsResolver) CodeInsights(ctx context.Context, args *struct {
	DateRange *string
	Grouping  *string
}) *adminanalytics.CodeInsights {
	return &adminanalytics.CodeInsights{Ctx: ctx, DateRange: *args.DateRange, Grouping: *args.Grouping, DB: r.db, Cache: r.cache}
}

/* Custom */

func (r *siteAnalyticsResolver) Custom(ctx context.Context, args *struct {
	DateRange *string
	Grouping  *string
	Events    []string
}) *adminanalytics.Custom {
	return &adminanalytics.Custom{Ctx: ctx, DateRange: *args.DateRange, Grouping: *args.Grouping, Events: args.Events, DB: r.db, Cache: r.cache}
}

/* All event names */

func (r *siteAnalyticsResolver) AllEventNames(ctx context.Context) ([]*string, error) {
	return adminanalytics.AllEventNames(ctx, r.db, r.cache)
}
