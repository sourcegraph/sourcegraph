package apitest

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/graph-gophers/graphql-go"
	gqlerrors "github.com/graph-gophers/graphql-go/errors"

	"github.com/sourcegraph/sourcegraph/internal/jsonc"
)

// MustExec uses Exec to execute the given query and calls t.Fatalf if Exec failed.
func MustExec(
	ctx context.Context,
	t testing.TB,
	s *graphql.Schema,
	in map[string]interface{},
	out interface{},
	query string,
) {
	t.Helper()
	if errs := Exec(ctx, t, s, in, out, query); len(errs) > 0 {
		t.Fatalf("unexpected graphql query errors: %v", errs)
	}
}

// Exec executes the given query with the given input in the given
// graphql.Schema. The response will be rendered into out.
func Exec(
	ctx context.Context,
	t testing.TB,
	s *graphql.Schema,
	in map[string]interface{},
	out interface{},
	query string,
) []*gqlerrors.QueryError {
	t.Helper()

	query = strings.Replace(query, "\t", "  ", -1)

	b, err := json.Marshal(in)
	if err != nil {
		t.Fatalf("failed to marshal input: %s", err)
	}

	var anonInput map[string]interface{}
	err = json.Unmarshal(b, &anonInput)
	if err != nil {
		t.Fatalf("failed to unmarshal input back: %s", err)
	}

	r := s.Exec(ctx, query, "", anonInput)
	if len(r.Errors) != 0 {
		return r.Errors
	}

	_, disableLog := os.LookupEnv("NO_GRAPHQL_LOG")

	if testing.Verbose() && !disableLog {
		t.Logf("\n---- GraphQL Query ----\n%s\n\nVars: %s\n---- GraphQL Result ----\n%s\n -----------", query, toJSON(t, in), r.Data)
	}

	if err := json.Unmarshal(r.Data, out); err != nil {
		t.Fatalf("failed to unmarshal graphql data: %v", err)
	}

	return nil
}

func toJSON(t testing.TB, v interface{}) string {
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}

	formatted, err := jsonc.Format(string(data), nil)
	if err != nil {
		t.Fatal(err)
	}

	return formatted
}
