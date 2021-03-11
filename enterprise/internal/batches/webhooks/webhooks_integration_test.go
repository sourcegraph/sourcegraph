package webhooks

import (
	"testing"

	ct "github.com/sourcegraph/sourcegraph/enterprise/internal/batches/testing"
	"github.com/sourcegraph/sourcegraph/internal/database/dbtesting"
)

func TestWebhooksIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	t.Parallel()

	db := dbtesting.GetDB(t)

	user := ct.CreateTestUser(t, db, false)

	t.Run("GitHubWebhook", testGitHubWebhook(db, user.ID))
	t.Run("BitbucketWebhook", testBitbucketWebhook(db, user.ID))
	t.Run("GitLabWebhook", testGitLabWebhook(db, user.ID))
}
