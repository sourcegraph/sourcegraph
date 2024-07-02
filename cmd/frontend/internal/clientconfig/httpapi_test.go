package clientconfig

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hexops/autogold/v2"
	"github.com/sourcegraph/log/logtest"

	"github.com/sourcegraph/sourcegraph/cmd/frontend/enterprise"
	"github.com/sourcegraph/sourcegraph/internal/actor"
	"github.com/sourcegraph/sourcegraph/internal/codeintel"
	"github.com/sourcegraph/sourcegraph/internal/conf"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/database/dbtest"
	"github.com/sourcegraph/sourcegraph/internal/license"
	"github.com/sourcegraph/sourcegraph/internal/licensing"
	"github.com/sourcegraph/sourcegraph/internal/observation"
	"github.com/sourcegraph/sourcegraph/schema"
)

func TestAPI(t *testing.T) {
	logger := logtest.Scoped(t)
	db := database.NewDB(logger, dbtest.NewDB(t))
	ctx := context.Background()

	// Enable Cody (and all other license features)
	oldLicensingMock := licensing.MockCheckFeature
	licensing.MockCheckFeature = func(feature licensing.Feature) error {
		return nil
	}
	t.Cleanup(func() { licensing.MockCheckFeature = oldLicensingMock })

	// Mock the site configuration
	truePtr := true
	falsePtr := false
	licenseKey := "theasdfkey"
	licenseAccessToken := license.GenerateLicenseKeyBasedAccessToken(licenseKey)
	conf.Mock(&conf.Unified{
		SiteConfiguration: schema.SiteConfiguration{
			CodyEnabled:     &truePtr,
			CodyPermissions: &falsePtr, // disable RBAC Cody permissions
			Completions: &schema.Completions{
				AccessToken: licenseAccessToken,
			},
		},
	})
	t.Cleanup(func() { conf.Mock(nil) })

	// Initialize the service
	autogold.Expect(nil).Equal(t, Init(
		ctx,
		observation.TestContextTB(t),
		db,
		codeintel.Services{},
		conf.DefaultClient(),
		&enterprise.Services{},
	))
	t.Cleanup(func() { singletonConfigService = nil })

	// Grab HTTP handlers
	handlers := NewHandlers(db, logger)

	// Note: all the mechanics of conf.GetConfigFeatures, RBAC cody access via cody.IsCodyEnabled,
	// and conf.GetCompletionsConfig are tested independently at their implementations. We really
	// only test that those properties are relayed correctly by the HTTP API here.

	t.Run("unauthenticated", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "", nil)
		recorder := httptest.NewRecorder()
		handlers.GetClientConfigHandler(recorder, req)

		autogold.Expect(int(401)).Equal(t, recorder.Code)
		autogold.Expect("Unauthorized\n").Equal(t, recorder.Body.String())
	})

	t.Run("authenticated_defaults", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "", nil)
		req = req.WithContext(
			actor.WithActor(ctx, &actor.Actor{
				UID: 99,
			}),
		)
		recorder := httptest.NewRecorder()
		handlers.GetClientConfigHandler(recorder, req)

		autogold.Expect(int(200)).Equal(t, recorder.Code)
		autogold.Expect(`{
    "v1": {
        "CodyEnabled": true,
        "ChatEnabled": true,
        "AutoCompleteEnabled": true,
        "CommandsEnabled": true,
        "AttributionEnabled": false,
        "SmartContextWindowEnabled": false,
        "ModelsAPIEnabled": false
    }
}
`).Equal(t, recorder.Body.String())
	})
}
