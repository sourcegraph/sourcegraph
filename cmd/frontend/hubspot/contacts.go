package hubspot

import (
	"errors"
	"fmt"
	"net/url"
	"reflect"
)

// CreateOrUpdateContact creates or updates a HubSpot contact (with email as primary key)
//
// The endpoint returns 200 with the contact's VID and an isNew field on success,
// or a 409 Conflict if we attempt to change a user's email address to a new one
// that is already taken
//
// http://developers.hubspot.com/docs/methods/contacts/create_or_update
func (c *Client) CreateOrUpdateContact(email string, params *ContactProperties) (*ContactResponse, error) {
	if c.hapiKey == "" {
		return nil, errors.New("HubSpot API key must be provided.")
	}
	var resp ContactResponse
	err := c.postJSON("CreateOrUpdateContact", c.baseContactURL(email), newAPIValues(params), &resp)
	return &resp, err
}

func (c *Client) baseContactURL(email string) *url.URL {
	q := url.Values{}
	q.Set("hapikey", c.hapiKey)

	return &url.URL{
		Scheme:   "https",
		Host:     "api.hubapi.com",
		Path:     "/contacts/v1/contact/createOrUpdate/email/" + email + "/",
		RawQuery: q.Encode(),
	}
}

// ContactProperties represent HubSpot user properties
type ContactProperties struct {
	UserID          string `json:"user_id"`
	IsServerAdmin   bool   `json:"is_server_admin"`
	LatestPing      int64  `json:"latest_ping"`
	AnonymousUserID string `json:"anonymous_user_id"`
	FirstSourceURL  string `json:"first_source_url"`
}

// ContactResponse represents HubSpot user properties returned
// after a CreateOrUpdate API call
type ContactResponse struct {
	VID   int32 `json:"vid"`
	IsNew bool  `json:"isNew"`
}

// newAPIValues converts a ContactProperties struct to a HubSpot API-compliant
// array of key-value pairs
func newAPIValues(h *ContactProperties) *apiProperties {
	apiProps := &apiProperties{}
	apiProps.set("user_id", h.UserID)
	apiProps.set("is_server_admin", h.IsServerAdmin)
	apiProps.set("latest_ping", h.LatestPing)
	apiProps.set("anonymous_user_id", h.AnonymousUserID)
	apiProps.set("first_source_url", h.FirstSourceURL)
	return apiProps
}

// apiProperties represents a list of HubSpot API-compliant key-value pairs
type apiProperties struct {
	Properties []*apiProperty `json:"properties"`
}

type apiProperty struct {
	Property string `json:"property"`
	Value    string `json:"value"`
}

func (h *apiProperties) set(property string, value interface{}) {
	if h.Properties == nil {
		h.Properties = make([]*apiProperty, 0)
	}
	if value != reflect.Zero(reflect.TypeOf(value)).Interface() {
		h.Properties = append(h.Properties, &apiProperty{Property: property, Value: fmt.Sprintf("%v", value)})
	}
}
