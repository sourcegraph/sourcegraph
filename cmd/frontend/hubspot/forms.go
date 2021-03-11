package hubspot

import "net/url"

// SubmitForm submits form data.  Form submissions return an empty
// body with status code 204 or 302 if submission was successful.
//
// `params` must be a map[string]string or a struct convertible to
// a URL querystring using query.Values(). The keys (or `url` tags
// in the struct) must be snake case, per HubSpot conventions.
//
// See https://developers.hubspot.com/docs/methods/forms/submit_form.
func (c *Client) SubmitForm(formID string, params interface{}) error {
	return c.postForm("SubmitForm", c.baseFormURL(), formID, params)
}

func (c *Client) baseFormURL() *url.URL {
	return &url.URL{
		Scheme: "https",
		Host:   "forms.hubspot.com",
		Path:   "/uploads/form/v2/" + c.portalID,
	}
}
