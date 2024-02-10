package backend

import (
	"context"
	"net/url"
	"strconv"

	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/randstring"
)

func MakeRandomHardToGuessPassword() string {
	return randstring.NewLen(36)
}

var MockMakePasswordResetURL func(ctx context.Context, userID int32, email string) (*url.URL, error)

func MakePasswordResetURL(ctx context.Context, db database.DB, userID int32, email string) (*url.URL, error) {
	if MockMakePasswordResetURL != nil {
		return MockMakePasswordResetURL(ctx, userID, email)
	}
	resetCode, err := db.Users().RenewPasswordResetCode(ctx, userID)
	if err != nil {
		return nil, err
	}
	query := url.Values{}
	query.Set("userID", strconv.Itoa(int(userID)))
	query.Set("code", resetCode)

	// This field will be used by the frontend for displaying the email on the password entry page
	query.Set("email", email)

	return &url.URL{Path: "/password-reset", RawQuery: query.Encode()}, nil
}
