package analytics

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path"
	"strconv"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	goauth2 "google.golang.org/api/oauth2/v2"

	"github.com/sourcegraph/log"

	"github.com/sourcegraph/sourcegraph/dev/sg/internal/secrets"
	"github.com/sourcegraph/sourcegraph/dev/sg/internal/std"
	"github.com/sourcegraph/sourcegraph/dev/sg/root"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

const setupNotice = `A one-time gsuite login is required to set up your identity for analytics.
This can be opted out with the -disable-analytics flag, the SG_DISABLE_ANALYTICS env var or by writing '{"email": <your email or "anonymous">}' to ~/.sourcegraph/whoami.json.

As part of this, some action may be required to make this process work:
	1. Your computer may ask to receive incoming connections.
		Please allow so the browser and sg can communicate.
	2. Please accept the browser access request.

This process will resume automatically.
`

func InitIdentity(ctx context.Context, out *std.Output, sec *secrets.Store) error {
	sgHome, err := root.GetSGHomePath()
	if err != nil {
		return err
	}

	whoamiPath := path.Join(sgHome, "whoami.json")

	if content, err := os.ReadFile(whoamiPath); err == nil {
		var whoami struct {
			Email string
		}
		if err := json.Unmarshal(content, &whoami); err != nil {
			return err
		}
		if whoami.Email != "" {
			return nil
		}
	}

	clientCredentials, err := sec.GetExternal(ctx, secrets.ExternalSecret{
		Project: secrets.LocalDevProject,
		// sg Google client credentials
		Name: "SG_GOOGLE_CREDS",
	})
	if err != nil {
		return errors.Wrap(err, "failed to get google client credentials")
	}

	config, err := google.ConfigFromJSON([]byte(clientCredentials), goauth2.UserinfoEmailScope)
	if err != nil {
		return errors.Wrap(err, "Unable to parse client secret file to config")
	}

	t := tokenHandlerImpl{config: config}

	redirectUrl, waitForCode, waitForError, err := handleAuthResponse()
	if err == nil {
		t.SetRedirectURL(redirectUrl)
	} else {
		return err
	}

	authURL := t.AuthCodeURL("state-token", oauth2.AccessTypeOffline)

	if err := t.OpenURL(authURL); err != nil {
		return err
	}

	out.WriteNoticef(setupNotice)

	err = <-waitForError
	if err != nil {
		return err
	}

	authCode := <-waitForCode

	token, err := config.Exchange(context.Background(), authCode)
	if err != nil {
		return err
	}

	var whoami struct {
		Email string `json:"email"`
	}
	resp, err := http.Get("https://www.googleapis.com/oauth2/v3/userinfo?access_token=" + token.AccessToken)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if err := json.NewDecoder(resp.Body).Decode(&whoami); err != nil {
		return err
	}

	b, err := json.Marshal(whoami)
	if err != nil {
		return err
	}

	return os.WriteFile(whoamiPath, b, 0o600)
}

type tokenHandlerImpl struct {
	config *oauth2.Config
}

func (th *tokenHandlerImpl) SetRedirectURL(url *url.URL) {
	th.config.RedirectURL = url.String()
}

func (th *tokenHandlerImpl) AuthCodeURL(state string, opts ...oauth2.AuthCodeOption) string {
	return th.config.AuthCodeURL(state, opts...)
}

func (th *tokenHandlerImpl) Exchange(ctx context.Context, code string, opts ...oauth2.AuthCodeOption) (*oauth2.Token, error) {
	return th.config.Exchange(ctx, code, opts...)
}

func (th *tokenHandlerImpl) OpenURL(url string) error {
	return exec.Command("open", url).Start()
}

func handleAuthResponse() (*url.URL, chan string, chan error, error) {
	codeReceiver := make(chan string, 1)
	errorReceiver := make(chan error, 1)

	socket, err := net.Listen("tcp", ":0")
	if err != nil {
		return nil, nil, nil, err
	}

	logger := log.Scoped("auth_handler")
	var server http.Server
	gracefulShutdown := false

	// Creates a handler to handle response
	handler := http.NewServeMux()
	handler.Handle("/oauth2/callback", http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		authCode := r.URL.Query().Get("code")
		if authCode == "" {
			errorReceiver <- errors.Errorf("Did not get authentication code from Google")
			return
		}
		rw.Header().Add("Content-Type", "text/plain")
		_, _ = rw.Write([]byte(`'sg' authentication complete. You may close this window.`))
		errorReceiver <- nil
		codeReceiver <- authCode
		gracefulShutdown = true
	}))

	server.Handler = handler

	go func() {
		defer socket.Close()
		if err := server.Serve(socket); err != nil {
			if !gracefulShutdown {
				logger.Error("failure to handle", log.Error(err))
			}
		}
	}()

	redirectUrl := url.URL{
		Host:   net.JoinHostPort("127.0.0.1", strconv.Itoa(socket.Addr().(*net.TCPAddr).Port)),
		Path:   "/oauth2/callback",
		Scheme: "http",
	}

	return &redirectUrl, codeReceiver, errorReceiver, nil
}
