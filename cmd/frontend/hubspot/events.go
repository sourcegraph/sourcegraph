package hubspot

import "net/url"

// LogEvent logs a user action or event. The response will have a status code of
// 200 with no data in the body
//
// http://developers.hubspot.com/docs/methods/enterprise_events/http_api
func (c *Client) LogEvent(email string, eventID string, params map[string]string) error {
	params["_a"] = c.portalID
	params["_n"] = eventID
	params["email"] = email
	err := c.get("LogEvent", c.baseEventURL(), email, params)
	if err != nil {
		return err
	}
	return nil
}

func (c *Client) baseEventURL() *url.URL {
	return &url.URL{
		Scheme: "https",
		Host:   "track.hubspot.com",
		Path:   "/v1/event",
	}
}
