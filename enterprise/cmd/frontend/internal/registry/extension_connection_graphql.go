package registry

import (
	"context"
	"strings"
	"unicode"

	"github.com/sourcegraph/sourcegraph/cmd/frontend/graphqlbackend"
	registry "github.com/sourcegraph/sourcegraph/cmd/frontend/registry/api"
	"github.com/sourcegraph/sourcegraph/internal/database/dbutil"
)

func init() {
	registry.ListLocalRegistryExtensions = listLocalRegistryExtensions
	registry.CountLocalRegistryExtensions = countLocalRegistryExtensions
}

func listLocalRegistryExtensions(ctx context.Context, db dbutil.DB, args graphqlbackend.RegistryExtensionConnectionArgs) ([]graphqlbackend.RegistryExtension, error) {
	if args.PrioritizeExtensionIDs != nil {
		ids := filterStripLocalExtensionIDs(*args.PrioritizeExtensionIDs)
		args.PrioritizeExtensionIDs = &ids
	}
	opt, err := toDBExtensionsListOptions(args)
	if err != nil {
		return nil, err
	}

	vs, err := dbExtensions{}.List(ctx, opt)
	if err != nil {
		return nil, err
	}
	if err := prefixLocalExtensionID(vs...); err != nil {
		return nil, err
	}

	releasesByExtensionID, err := getLatestForBatch(ctx, vs)
	if err != nil {
		return nil, err
	}
	var ys []graphqlbackend.RegistryExtension
	for _, v := range vs {
		ys = append(ys, &extensionDBResolver{db: db, v: v, r: releasesByExtensionID[v.ID]})
	}
	return ys, nil
}

func countLocalRegistryExtensions(ctx context.Context, db dbutil.DB, args graphqlbackend.RegistryExtensionConnectionArgs) (int, error) {
	opt, err := toDBExtensionsListOptions(args)
	if err != nil {
		return 0, err
	}
	return dbExtensions{}.Count(ctx, opt)
}

func toDBExtensionsListOptions(args graphqlbackend.RegistryExtensionConnectionArgs) (dbExtensionsListOptions, error) {
	var opt dbExtensionsListOptions
	args.ConnectionArgs.Set(&opt.LimitOffset)
	if args.Publisher != nil {
		p, err := unmarshalRegistryPublisherID(*args.Publisher)
		if err != nil {
			return opt, err
		}
		opt.Publisher.UserID = p.userID
		opt.Publisher.OrgID = p.orgID
	}
	if args.Query != nil {
		opt.Query, opt.Category, opt.Tag = parseExtensionQuery(*args.Query)
	}
	if args.PrioritizeExtensionIDs != nil {
		opt.PrioritizeExtensionIDs = *args.PrioritizeExtensionIDs
	}
	return opt, nil
}

// parseExtensionQuery parses an extension registry query consisting of terms and the operators
// `category:"My category"`, `tag:"mytag"`, #installed, #enabled, and #disabled.
//
// This is an intentionally simple, unoptimized parser.
func parseExtensionQuery(q string) (text, category, tag string) {
	// Tokenize.
	var lastQuote rune
	tokens := strings.FieldsFunc(q, func(c rune) bool {
		switch {
		case c == lastQuote:
			lastQuote = rune(0)
			return false
		case lastQuote != rune(0):
			return false
		case c == '"' || c == '\'':
			lastQuote = c
			return false
		default:
			return unicode.IsSpace(c)
		}
	})

	unquoteValue := func(s string) string {
		return strings.Trim(s, `"'`)
	}

	var textTokens []string
	for _, tok := range tokens {
		if strings.HasPrefix(tok, "category:") {
			category = unquoteValue(strings.TrimPrefix(tok, "category:"))
		} else if strings.HasPrefix(tok, "tag:") {
			tag = unquoteValue(strings.TrimPrefix(tok, "tag:"))
		} else if tok == "#installed" || tok == "#enabled" || tok == "#disabled" {
			// Ignore so that the client can implement these in post-processing.
		} else {
			textTokens = append(textTokens, tok)
		}
	}
	return strings.Join(textTokens, " "), category, tag
}

// filterStripLocalExtensionIDs filters to local extension IDs and strips the
// host prefix.
func filterStripLocalExtensionIDs(extensionIDs []string) []string {
	prefix := registry.GetLocalRegistryExtensionIDPrefix()
	local := []string{}
	for _, id := range extensionIDs {
		parts := strings.SplitN(id, "/", 3)
		if prefix != nil && len(parts) == 3 && parts[0] == *prefix {
			local = append(local, parts[1]+"/"+parts[2])
		} else if (prefix == nil || *prefix == "") && len(parts) == 2 {
			local = append(local, id)
		}
	}
	return local
}
