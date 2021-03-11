package batches

import (
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestChangesetSpecUnmarshalValidate(t *testing.T) {
	tests := []struct {
		name    string
		rawSpec string
		err     string
	}{
		{
			name: "valid ExistingChangesetReference",
			rawSpec: `{
				"baseRepository": "graphql-id",
				"externalID": "1234"
			}`,
		},
		{
			name: "valid GitBranchChangesetDescription",
			rawSpec: `{
				"baseRepository": "graphql-id",
				"baseRef": "refs/heads/master",
				"baseRev": "d34db33f",
				"headRef": "refs/heads/my-branch",
				"headRepository": "graphql-id",
				"title": "my title",
				"body": "my body",
				"published": false,
				"commits": [{
				  "message": "commit message",
				  "diff": "the diff",
				  "authorName": "Mary McButtons",
				  "authorEmail": "mary@example.com"
				}]
			}`,
		},
		{
			name: "missing fields in GitBranchChangesetDescription",
			rawSpec: `{
				"baseRepository": "graphql-id",
				"baseRef": "refs/heads/master",
				"headRef": "refs/heads/my-branch",
				"headRepository": "graphql-id",
				"title": "my title",
				"published": false,
				"commits": [{
				  "diff": "the diff",
				  "authorName": "Mary McButtons",
				  "authorEmail": "mary@example.com"
				}]
			}`,
			err: "4 errors occurred:\n\t* Must validate one and only one schema (oneOf)\n\t* baseRev is required\n\t* body is required\n\t* commits.0: message is required\n\n",
		},
		{
			name: "missing fields in ExistingChangesetReference",
			rawSpec: `{
				"baseRepository": "graphql-id"
			}`,
			err: "2 errors occurred:\n\t* Must validate one and only one schema (oneOf)\n\t* externalID is required\n\n",
		},
		{
			name: "headRepository in GitBranchChangesetDescription does not match baseRepository",
			rawSpec: `{
				"baseRepository": "graphql-id",
				"baseRef": "refs/heads/master",
				"baseRev": "d34db33f",
				"headRef": "refs/heads/my-branch",
				"headRepository": "graphql-id999999",
				"title": "my title",
				"body": "my body",
				"published": false,
				"commits": [{
				  "message": "commit message",
				  "diff": "the diff",
				  "authorName": "Mary McButtons",
				  "authorEmail": "mary@example.com"
				}]
			}`,
			err: ErrHeadBaseMismatch.Error(),
		},
		{
			name: "too many commits in GitBranchChangesetDescription",
			rawSpec: `{
				"baseRepository": "graphql-id",
				"baseRef": "refs/heads/master",
				"baseRev": "d34db33f",
				"headRef": "refs/heads/my-branch",
				"headRepository": "graphql-id",
				"title": "my title",
				"body": "my body",
				"published": false,
				"commits": [
				  {
				    "message": "commit message",
					"diff": "the diff",
					"authorName": "Mary McButtons",
					"authorEmail": "mary@example.com"
				  },
                  {
				    "message": "commit message2",
					"diff": "the diff2",
					"authorName": "Mary McButtons",
					"authorEmail": "mary@example.com"
				  }
				]
			}`,
			err: "2 errors occurred:\n\t* Must validate one and only one schema (oneOf)\n\t* commits: Array must have at most 1 items\n\n",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			spec := &ChangesetSpec{RawSpec: tc.rawSpec}
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
