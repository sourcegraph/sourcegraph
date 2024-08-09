package graphqlbackend

import (
	"context"

	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/dotcom"
	"github.com/sourcegraph/sourcegraph/internal/search"
	"github.com/sourcegraph/sourcegraph/internal/search/job"
	"github.com/sourcegraph/sourcegraph/internal/search/job/jobutil"
	"github.com/sourcegraph/sourcegraph/internal/search/job/printer"
	"github.com/sourcegraph/sourcegraph/internal/search/query"
	"github.com/sourcegraph/sourcegraph/internal/settings"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

// Refer to SearchQueryOutputPhase in GQL definitions.
const (
	ParseTree = "PARSE_TREE"
	JobTree   = "JOB_TREE"
)

// Refer to SearchQueryOutputFormat in GQL definitions.
const (
	Json    = "JSON"
	Sexp    = "SEXP"
	Mermaid = "MERMAID"
)

// Refer to SearchQueryOutputVerbosity in GQL definitions.
const (
	Minimal = "MINIMAL"
	Basic   = "BASIC"
	Maximal = "MAXIMAL"
)

type args struct {
	Query           string
	PatternType     string
	OutputPhase     string
	OutputFormat    string
	OutputVerbosity string
}

func (r *schemaResolver) ParseSearchQuery(ctx context.Context, args *args) (string, error) {
	var searchType query.SearchType
	switch args.PatternType {
	case "literal":
		searchType = query.SearchTypeLiteral
	case "structural":
		searchType = query.SearchTypeStructural
	case "regexp", "regex":
		searchType = query.SearchTypeRegex
	default:
		searchType = query.SearchTypeLiteral
	}

	switch args.OutputPhase {
	case ParseTree:
		return outputParseTree(searchType, args)
	case JobTree:
		return outputJobTree(ctx, searchType, args, r.db)
	}
	return "", nil
}

func outputParseTree(searchType query.SearchType, args *args) (string, error) {
	plan, err := query.Pipeline(query.Init(args.Query, searchType))
	if err != nil {
		return "", err
	}

	if args.OutputFormat != Json || args.OutputVerbosity != Basic {
		return "", errors.New("unsupported output options for PARSE_TREE, only JSON output with BASIC verbosity is supported")
	}
	jsonString, err := query.ToJSON(plan.ToQ())
	if err != nil {
		return "", err
	}
	return jsonString, nil
}

func outputJobTree(
	ctx context.Context,
	searchType query.SearchType,
	args *args,
	db database.DB,
) (string, error) {
	plan, err := query.Pipeline(query.Init(args.Query, searchType))
	if err != nil {
		return "", err
	}

	settings, err := settings.CurrentUserFinal(ctx, db)
	if err != nil {
		return "", err
	}

	inputs := &search.Inputs{
		UserSettings:        settings,
		PatternType:         searchType,
		Protocol:            search.Streaming,
		Features:            search.FeaturesFromContext(ctx),
		OnSourcegraphDotCom: dotcom.SourcegraphDotComMode(),
	}
	j, err := jobutil.NewPlanJob(inputs, plan)
	if err != nil {
		return "", err
	}

	var verbosity job.Verbosity
	switch args.OutputVerbosity {
	case Minimal:
		verbosity = job.VerbosityNone
	case Basic:
		verbosity = job.VerbosityBasic
	case Maximal:
		verbosity = job.VerbosityMax
	}

	switch args.OutputFormat {
	case Json:
		jsonString := printer.JSONVerbose(j, verbosity)
		return jsonString, nil
	case Sexp:
		sexpString := printer.SexpVerbose(j, verbosity, true)
		return sexpString, nil
	case Mermaid:
		mermaidString := printer.MermaidVerbose(j, verbosity)
		return mermaidString, nil
	}
	return "", nil
}
