package background

import (
	"context"
	"testing"

	"github.com/sourcegraph/sourcegraph/enterprise/internal/batches/store"
	ct "github.com/sourcegraph/sourcegraph/enterprise/internal/batches/testing"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/database/dbtesting"
	"github.com/sourcegraph/sourcegraph/internal/extsvc"
	"github.com/sourcegraph/sourcegraph/internal/extsvc/auth"
)

func TestSSHMigrator(t *testing.T) {
	ctx := context.Background()
	db := dbtesting.GetDB(t)
	user := ct.CreateTestUser(t, db, false)

	cstore := store.New(db)

	migrator := &sshMigrator{cstore}
	progress, err := migrator.Progress(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if have, want := progress, 1.0; have != want {
		t.Fatalf("got invalid progress with no DB entries, want=%f have=%f", want, have)
	}

	oauth := &auth.OAuthBearerToken{Token: "test"}
	credential, err := cstore.UserCredentials().Create(ctx, database.UserCredentialScope{
		Domain:              database.UserCredentialDomainBatches,
		UserID:              user.ID,
		ExternalServiceType: extsvc.TypeGitHub,
		ExternalServiceID:   "https://github.com/",
	}, oauth)
	if err != nil {
		t.Fatal(err)
	}

	progress, err = migrator.Progress(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if have, want := progress, 0.0; have != want {
		t.Fatalf("got invalid progress with one unmigrated entry, want=%f have=%f", want, have)
	}

	if err := migrator.Up(ctx); err != nil {
		t.Fatal(err)
	}

	progress, err = migrator.Progress(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if have, want := progress, 1.0; have != want {
		t.Fatalf("got invalid progress after up migration, want=%f have=%f", want, have)
	}

	{
		migratedCredential, err := cstore.UserCredentials().GetByID(ctx, credential.ID)
		if err != nil {
			t.Fatal(err)
		}
		if have, want := migratedCredential.Domain, credential.Domain; have != want {
			t.Fatalf("invalid Domain after migration, want=%q have=%q", want, have)
		}
		if have, want := migratedCredential.CreatedAt, credential.CreatedAt; have != want {
			t.Fatalf("invalid CreatedAt after migration, want=%q have=%q", want, have)
		}
		if have, want := migratedCredential.ExternalServiceID, credential.ExternalServiceID; have != want {
			t.Fatalf("invalid ExternalServiceID after migration, want=%q have=%q", want, have)
		}
		if have, want := migratedCredential.ExternalServiceType, credential.ExternalServiceType; have != want {
			t.Fatalf("invalid ExternalServiceType after migration, want=%q have=%q", want, have)
		}
		if have, want := migratedCredential.ID, credential.ID; have != want {
			t.Fatalf("invalid ID after migration, want=%d have=%d", want, have)
		}
		if have, want := migratedCredential.UserID, credential.UserID; have != want {
			t.Fatalf("invalid UserID after migration, want=%d have=%d", want, have)
		}
		switch c := migratedCredential.Credential.(type) {
		case *auth.OAuthBearerTokenWithSSH:
			if have, want := c.Token, oauth.Token; have != want {
				t.Fatalf("invalid token stored in migrated credential, want=%q have=%q", want, have)
			}
			if c.Passphrase == "" || c.PrivateKey == "" || c.PublicKey == "" {
				t.Fatal("ssh keypair is not complete")
			}
		default:
			t.Fatalf("invalid type of migrated credential: %T", migratedCredential.Credential)
		}
	}

	if err := migrator.Down(ctx); err != nil {
		t.Fatal(err)
	}

	progress, err = migrator.Progress(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if have, want := progress, 0.0; have != want {
		t.Fatalf("got invalid progress after down migration, want=%f have=%f", want, have)
	}

	{
		migratedCredential, err := cstore.UserCredentials().GetByID(ctx, credential.ID)
		if err != nil {
			t.Fatal(err)
		}
		if have, want := migratedCredential.Domain, credential.Domain; have != want {
			t.Fatalf("invalid Domain after down migration, want=%q have=%q", want, have)
		}
		if have, want := migratedCredential.CreatedAt, credential.CreatedAt; have != want {
			t.Fatalf("invalid CreatedAt after down migration, want=%q have=%q", want, have)
		}
		if have, want := migratedCredential.ExternalServiceID, credential.ExternalServiceID; have != want {
			t.Fatalf("invalid ExternalServiceID after down migration, want=%q have=%q", want, have)
		}
		if have, want := migratedCredential.ExternalServiceType, credential.ExternalServiceType; have != want {
			t.Fatalf("invalid ExternalServiceType after down migration, want=%q have=%q", want, have)
		}
		if have, want := migratedCredential.ID, credential.ID; have != want {
			t.Fatalf("invalid ID after down migration, want=%d have=%d", want, have)
		}
		if have, want := migratedCredential.UserID, credential.UserID; have != want {
			t.Fatalf("invalid UserID after down migration, want=%d have=%d", want, have)
		}
		switch c := migratedCredential.Credential.(type) {
		case *auth.OAuthBearerToken:
			if have, want := c.Token, oauth.Token; have != want {
				t.Fatalf("invalid token stored in migrated credential, want=%q have=%q", want, have)
			}
		default:
			t.Fatalf("invalid type of migrated credential: %T", migratedCredential.Credential)
		}
	}
}
