package scim

import (
	"context"
	"net/http"
	"testing"

	"github.com/elimity-com/scim"
	"github.com/sourcegraph/sourcegraph/internal/observation"
	"github.com/stretchr/testify/assert"
)

func TestUserResourceHandler_Create(t *testing.T) {
	db := getMockDB()
	userResourceHandler := NewUserResourceHandler(context.Background(), &observation.TestContext, db)
	testCases := []struct {
		name     string
		username string
		testFunc func(t *testing.T, usernameInDB string, usernameInResource string, err error)
	}{
		{
			name:     "create user with new username",
			username: "user5",
			testFunc: func(t *testing.T, usernameInDB string, usernameInResource string, err error) {
				assert.Equal(t, "user5", usernameInDB)
				assert.Equal(t, "user5", usernameInResource)
			},
		},
		{
			name:     "create user with existing username",
			username: "user4",
			testFunc: func(t *testing.T, usernameInDB string, usernameInResource string, err error) {
				assert.Len(t, usernameInDB, 5+1+5) // user4-abcde
				assert.Equal(t, "user4", usernameInResource)
			},
		},
		{
			name:     "create user with email address as the username",
			username: "test@company.com",
			testFunc: func(t *testing.T, usernameInDB string, usernameInResource string, err error) {
				assert.Equal(t, "test", usernameInDB)
				assert.Equal(t, "test@company.com", usernameInResource)
			},
		},
		{
			name:     "create user with email address as a duplicate username",
			username: "user4@company.com",
			testFunc: func(t *testing.T, usernameInDB string, usernameInResource string, err error) {
				assert.Len(t, usernameInDB, 5+1+5) // user4-abcde
				assert.Equal(t, "user4@company.com", usernameInResource)
			},
		},
		{
			name:     "create user with empty username",
			username: "",
			testFunc: func(t *testing.T, usernameInDB string, usernameInResource string, err error) {
				assert.Len(t, usernameInDB, 5) // abcde
				assert.Equal(t, "", usernameInResource)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			userRes, err := userResourceHandler.Create(&http.Request{}, createUserResourceAttributes(tc.username))
			assert.NoError(t, err)
			newUser, err := db.Users().GetByID(context.Background(), 5)
			assert.NoError(t, err)
			tc.testFunc(t, newUser.Username, userRes.Attributes["userName"].(string), err)
			_ = db.Users().Delete(context.Background(), 5)
		})
	}

}

func createUserResourceAttributes(username string) scim.ResourceAttributes {
	return scim.ResourceAttributes{
		"userName": username,
		"name": map[string]interface{}{
			"givenName":  "First",
			"middleName": "Middle",
			"familyName": "Last",
		},
		"emails": []interface{}{
			map[string]interface{}{
				"value":   "a@b.c",
				"primary": true,
			},
		},
	}
}
