package store

import (
	"regexp"

	"github.com/keegancsmith/sqlf"

	"github.com/sourcegraph/sourcegraph/enterprise/internal/batches/search"
)

// textSearchTermToClause generates a WHERE clause that can be used in a query
// to represent searching for the given term over the given fields.
//
// Note that there must be at least one field: failing to include any fields
// will likely result in broken queries!
func textSearchTermToClause(term search.TextSearchTerm, fields ...*sqlf.Query) *sqlf.Query {
	// The general SQL query format for a positive query is:
	//
	// (field1 ~* value OR field2 ~* value)
	//
	// For negative queries, we negate both the regex and boolean
	//
	// (field !~* value AND field !~* value)
	//
	// Note that we're using the case insensitive versions of the regex
	// operators here.
	var boolOp string
	var textOp *sqlf.Query
	if term.Not {
		boolOp = "AND"
		textOp = sqlf.Sprintf("!~*")
	} else {
		boolOp = "OR"
		textOp = sqlf.Sprintf("~*")
	}

	// Since we're using regular expressions here, we need to ensure the search
	// term is correctly quoted to avoid issues with escape characters having
	// unexpected meaning in searches.
	quoted := regexp.QuoteMeta(term.Term)

	// Put together each field.
	exprs := make([]*sqlf.Query, len(fields))
	for i, field := range fields {
		// The ugly ('\m'||%s||'\M') construction gives us a regex that only
		// matches on word boundaries.
		exprs[i] = sqlf.Sprintf(`%s %s ('\m'||%s||'\M')`, field, textOp, quoted)
	}

	return sqlf.Sprintf("(%s)", sqlf.Join(exprs, boolOp))
}
