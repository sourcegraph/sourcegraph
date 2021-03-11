package httptestutil

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/url"

	"strings"
)

func NewTest(h http.Handler) *Client {
	return &Client{http.Client{Transport: handlerTransport{h}}}
}

type handlerTransport struct {
	http.Handler
}

func (t handlerTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	rw := httptest.NewRecorder()
	rw.Body = new(bytes.Buffer)
	if req.Body == nil {
		// For server requests the Request Body is always non-nil.
		req.Body = ioutil.NopCloser(bytes.NewReader(nil))
	}
	t.Handler.ServeHTTP(rw, req)
	return rw.Result(), nil
}

type Client struct{ http.Client }

// Get buffers the response body so that callers need not call
// resp.Body.Close().
func (c *Client) Get(url string) (*http.Response, error) {
	req, _ := http.NewRequest("GET", url, nil)
	return c.Do(req)
}

// Do buffers the response body so that callers need not call
// resp.Body.Close().
func (c *Client) Do(req *http.Request) (*http.Response, error) {
	resp, err := c.Client.Do(req)
	if resp != nil && resp.Body != nil {
		defer resp.Body.Close()
		body, _ := ioutil.ReadAll(resp.Body)
		resp.Body = ioutil.NopCloser(bytes.NewReader(body))
	}
	if err != nil {
		return resp, err
	}
	return resp, err
}

// DoOK checks that the response is HTTP 200.
func (c *Client) DoOK(req *http.Request) (*http.Response, error) {
	resp, err := c.Do(req)
	if resp != nil && resp.StatusCode != http.StatusOK {
		err = fmt.Errorf("Do %s %s: HTTP %d (%s)", req.URL, req.Method, resp.StatusCode, resp.Status)
	}
	return resp, err
}

// GetOK checks that the response is HTTP 200.
func (c *Client) GetOK(url string) (*http.Response, error) {
	req, _ := http.NewRequest("GET", url, nil)
	return c.DoOK(req)
}

// PostOK checks that the response is HTTP 200.
func (c *Client) PostOK(url string, body io.Reader) (*http.Response, error) {
	req, _ := http.NewRequest("POST", url, body)
	return c.DoOK(req)
}

func (c Client) DoNoFollowRedirects(req *http.Request) (*http.Response, error) {
	noRedir := errors.New("x")
	c.CheckRedirect = func(r *http.Request, via []*http.Request) error { return noRedir }
	resp, err := c.Do(req)
	if urlErr, ok := err.(*url.Error); ok && urlErr != nil {
		if urlErr.Err == noRedir {
			err = nil
		}
	}
	return resp, err
}

func (c Client) GetNoFollowRedirects(url_ string) (*http.Response, error) {
	req, err := http.NewRequest("GET", url_, nil)
	if err != nil {
		return nil, err
	}
	return c.DoNoFollowRedirects(req)
}

func (c *Client) GetJSON(url string, v interface{}) error {
	return c.DoJSON("GET", url, nil, v)
}

func (c *Client) DoJSON(method, url string, in, out interface{}) error {
	var reqBody io.Reader
	if in != nil {
		b, err := json.Marshal(in)
		if err != nil {
			return err
		}
		reqBody = bytes.NewReader(b)
	}

	req, err := http.NewRequest(method, url, reqBody)
	if err != nil {
		return err
	}
	req.Header.Set("content-type", "application/json; charset=utf-8")

	resp, err := c.DoOK(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if ct := resp.Header.Get("content-type"); !strings.HasPrefix(ct, "application/json") {
		return fmt.Errorf("content type %q is not JSON", ct)
	}
	if out != nil {
		if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
			return err
		}
	}
	return nil
}

func (c *Client) PostFormNoFollowRedirects(url string, data url.Values) (*http.Response, error) {
	req, err := http.NewRequest("POST", url, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("content-type", "application/x-www-form-urlencoded")
	return c.DoNoFollowRedirects(req)
}
