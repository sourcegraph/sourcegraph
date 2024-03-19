package graphqlbackend

import (
	"context"
	"testing"

	mockrequire "github.com/derision-test/go-mockgen/v2/testutil/require"
	gqlerrors "github.com/graph-gophers/graphql-go/errors"

	"github.com/sourcegraph/sourcegraph/internal/actor"
	"github.com/sourcegraph/sourcegraph/internal/database/dbmocks"
	ts "github.com/sourcegraph/sourcegraph/internal/temporarysettings"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

func TestTemporarySettingsNotSignedIn(t *testing.T) {
	t.Parallel()

	db := dbmocks.NewMockDB()
	tss := dbmocks.NewMockTemporarySettingsStore()
	db.TemporarySettingsFunc.SetDefaultReturn(tss)

	wantErr := errors.New("not authenticated")

	RunTests(t, []*Test{
		{
			// No actor set on context.
			Context: context.Background(),
			Schema:  mustParseGraphQLSchema(t, db),
			Query: `
				query {
					temporarySettings {
						contents
					}
				}
			`,
			ExpectedResult: "null",
			ExpectedErrors: []*gqlerrors.QueryError{
				{
					Path:          []any{"temporarySettings"},
					Message:       wantErr.Error(),
					ResolverError: wantErr,
				},
			},
		},
	})

	mockrequire.NotCalled(t, tss.GetTemporarySettingsFunc)
}

func TestTemporarySettings(t *testing.T) {
	t.Parallel()

	tss := dbmocks.NewMockTemporarySettingsStore()
	tss.GetTemporarySettingsFunc.SetDefaultHook(func(ctx context.Context, userID int32) (*ts.TemporarySettings, error) {
		if userID != 1 {
			t.Fatalf("should call GetTemporarySettings with userID=1, got=%d", userID)
		}
		return &ts.TemporarySettings{Contents: "{\"search.collapsedSidebarSections\": {\"types\": false}}"}, nil
	})
	db := dbmocks.NewMockDB()
	db.TemporarySettingsFunc.SetDefaultReturn(tss)

	RunTests(t, []*Test{
		{
			Context: actor.WithActor(context.Background(), actor.FromUser(1)),
			Schema:  mustParseGraphQLSchema(t, db),
			Query: `
				query {
					temporarySettings {
						contents
					}
				}
			`,
			ExpectedResult: `
				{
					"temporarySettings": {
						"contents": "{\"search.collapsedSidebarSections\": {\"types\": false}}"
					}
				}
			`,
		},
	})

	mockrequire.Called(t, tss.GetTemporarySettingsFunc)
}

func TestOverwriteTemporarySettingsNotSignedIn(t *testing.T) {
	t.Parallel()

	db := dbmocks.NewMockDB()
	tss := dbmocks.NewMockTemporarySettingsStore()
	db.TemporarySettingsFunc.SetDefaultReturn(tss)

	wantErr := errors.New("not authenticated")

	RunTests(t, []*Test{
		{
			// No actor set on context.
			Context: context.Background(),
			Schema:  mustParseGraphQLSchema(t, db),
			Query: `
				mutation ModifyTemporarySettings {
					overwriteTemporarySettings(
						contents: "{\"search.collapsedSidebarSections\": []}"
					) {
						alwaysNil
					}
				}
			`,
			ExpectedResult: "null",
			ExpectedErrors: []*gqlerrors.QueryError{
				{
					Path:          []any{"overwriteTemporarySettings"},
					Message:       wantErr.Error(),
					ResolverError: wantErr,
				},
			},
		},
	})

	mockrequire.NotCalled(t, tss.OverwriteTemporarySettingsFunc)
}

func TestOverwriteTemporarySettings(t *testing.T) {
	t.Parallel()

	db := dbmocks.NewMockDB()
	tss := dbmocks.NewMockTemporarySettingsStore()
	tss.OverwriteTemporarySettingsFunc.SetDefaultHook(func(ctx context.Context, userID int32, contents string) error {
		if userID != 1 {
			t.Fatalf("should call OverwriteTemporarySettings with userID=1, got=%d", userID)
		}
		return nil
	})
	db.TemporarySettingsFunc.SetDefaultReturn(tss)

	RunTests(t, []*Test{
		{
			Context: actor.WithActor(context.Background(), actor.FromUser(1)),
			Schema:  mustParseGraphQLSchema(t, db),
			Query: `
				mutation OverwriteTemporarySettings {
					overwriteTemporarySettings(
						contents: "{\"search.collapsedSidebarSections\": []}"
					) {
						alwaysNil
					}
				}
			`,
			ExpectedResult: "{\"overwriteTemporarySettings\":{\"alwaysNil\":null}}",
		},
	})

	mockrequire.Called(t, tss.OverwriteTemporarySettingsFunc)
}

func TestEditTemporarySettings(t *testing.T) {
	t.Parallel()

	db := dbmocks.NewMockDB()
	tss := dbmocks.NewMockTemporarySettingsStore()
	tss.EditTemporarySettingsFunc.SetDefaultHook(func(ctx context.Context, userID int32, settingsToEdit string) error {
		if userID != 1 {
			t.Fatalf("should call OverwriteTemporarySettings with userID=1, got=%d", userID)
		}
		return nil
	})
	db.TemporarySettingsFunc.SetDefaultReturn(tss)

	RunTests(t, []*Test{
		{
			Context: actor.WithActor(context.Background(), actor.FromUser(1)),
			Schema:  mustParseGraphQLSchema(t, db),
			Query: `
				mutation EditTemporarySettings {
					editTemporarySettings(
						settingsToEdit: "{\"search.collapsedSidebarSections\": []}"
					) {
						alwaysNil
					}
				}
			`,
			ExpectedResult: "{\"editTemporarySettings\":{\"alwaysNil\":null}}",
		},
	})

	mockrequire.Called(t, tss.EditTemporarySettingsFunc)
}
