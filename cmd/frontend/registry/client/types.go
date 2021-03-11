package client

import "time"

// Extension describes an extension in the extension registry.
//
// It is the external form of
// github.com/sourcegraph/sourcegraph/cmd/frontend/types.RegistryExtension (which is the
// internal DB type). These types should generally be kept in sync, but registry.Extension updates
// require backcompat.
type Extension struct {
	UUID        string    `json:"uuid"`
	ExtensionID string    `json:"extensionID"`
	Publisher   Publisher `json:"publisher"`
	Name        string    `json:"name"`
	Manifest    *string   `json:"manifest"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
	PublishedAt time.Time `json:"publishedAt"`
	URL         string    `json:"url"`

	// RegistryURL is the URL of the remote registry that this extension was retrieved from. It is
	// not set by package registry.
	RegistryURL string `json:"-"`
}

// Publisher describes a publisher in the extension registry.
type Publisher struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}
