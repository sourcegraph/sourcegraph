package gqltestutil

import (
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

// CreateAccessToken creates a new access token with given note and scopes for the
// authenticated user. It returns the new access token created.
func (c *Client) CreateAccessToken(note string, scopes []string, durationSeconds *int) (string, error) {
	const query = `
mutation CreateAccessToken($user: ID!, $scopes: [String!]!, $note: String!, $durationSeconds: Int) {
	createAccessToken(user: $user, scopes: $scopes, note: $note, durationSeconds: $durationSeconds) {
		token
	}
}
`
	variables := map[string]any{
		"user":            c.userID,
		"scopes":          scopes,
		"note":            note,
		"durationSeconds": durationSeconds,
	}
	var resp struct {
		Data struct {
			CreateAccessToken struct {
				Token string `json:"token"`
			} `json:"createAccessToken"`
		} `json:"data"`
	}
	err := c.GraphQL("", query, variables, &resp)
	if err != nil {
		return "", errors.Wrap(err, "request GraphQL")
	}
	return resp.Data.CreateAccessToken.Token, nil
}

// DeleteAccessToken deletes the given access token of the authenticated user.
func (c *Client) DeleteAccessToken(token string) error {
	const query = `
mutation DeleteAccessToken($token: String!) {
	deleteAccessToken(byToken: $token) {
		alwaysNil
	}
}
`
	variables := map[string]any{
		"token": token,
	}
	err := c.GraphQL("", query, variables, nil)
	if err != nil {
		return errors.Wrap(err, "request GraphQL")
	}
	return nil
}
