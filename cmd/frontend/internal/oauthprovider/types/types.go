package types

import "time"

type OAuthClientApplication struct {
	ID           int64
	Name         string
	Description  string
	RedirectURL  string
	ClientID     string
	ClientSecret string
	CreatedAt    time.Time
	UpdatedAt    time.Time
	Creator      int32
}
