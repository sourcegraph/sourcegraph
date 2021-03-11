package graphqlbackend

import _ "embed"

// Schema is the raw graqhql schema.
//go:embed schema.graphql
var Schema string
