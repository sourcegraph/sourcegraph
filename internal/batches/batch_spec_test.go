package batches

import (
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestBatchSpecUnmarshalValidate(t *testing.T) {
	tests := []struct {
		name    string
		rawSpec string
		err     string
	}{
		{
			name: "valid",
			rawSpec: `{
				"name": "my-unique-name",
				"description": "My description",
				"on": [
				    {"repositoriesMatchingQuery": "lang:go func main"},
					{"repository": "github.com/sourcegraph/src-cli"}
				],
				"steps": [
				{
					"run": "echo 'foobar'",
					"container": "alpine",
					"env": {
						"PATH": "/work/foobar:$PATH"
					}
				}
				],
				"changesetTemplate": {
					"title": "Hello World",
					"body": "My first batch change!",
					"branch": "hello-world",
					"commit": {
						"message": "Append Hello World to all README.md files"
					},
					"published": false
				}
			}`,
		},
		{
			name: "valid YAML",
			rawSpec: `
name: my-unique-name
description: My description
on:
- repositoriesMatchingQuery: lang:go func main
- repository: github.com/sourcegraph/src-cli
steps:
- run: echo 'foobar'
  container: alpine
  env:
    PATH: "/work/foobar:$PATH"
changesetTemplate:
  title: Hello World
  body: My first batch change!
  branch: hello-world
  commit:
    message: Append Hello World to all README.md files
  published: false
`,
		},
		{
			name: "invalid name",
			rawSpec: `{
				"name": "this contains spaces",
				"description": "My description",
				"on": [
				    {"repositoriesMatchingQuery": "lang:go func main"},
					{"repository": "github.com/sourcegraph/src-cli"}
				],
				"steps": [
				{
					"run": "echo 'foobar'",
					"container": "alpine",
					"env": {
						"PATH": "/work/foobar:$PATH"
					}
				}
				],
				"changesetTemplate": {
					"title": "Hello World",
					"body": "My first batch change!",
					"branch": "hello-world",
					"commit": {
						"message": "Append Hello World to all README.md files"
					},
					"published": false
				}
			}`,
			err: "1 error occurred:\n\t* name: Does not match pattern '^[\\w.-]+$'\n\n",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			spec := &BatchSpec{RawSpec: tc.rawSpec}
			haveErr := fmt.Sprintf("%v", spec.UnmarshalValidate())
			if haveErr == "<nil>" {
				haveErr = ""
			}
			if diff := cmp.Diff(tc.err, haveErr); diff != "" {
				t.Fatalf("unexpected response (-want +got):\n%s", diff)
			}
		})
	}
}
