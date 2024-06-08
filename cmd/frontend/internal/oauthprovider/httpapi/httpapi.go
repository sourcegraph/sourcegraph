package httpapi

import (
	"context"
	"log"
	"net/http"

	"github.com/go-oauth2/oauth2/v4"
	"github.com/go-oauth2/oauth2/v4/errors"
	"github.com/go-oauth2/oauth2/v4/manage"
	"github.com/go-oauth2/oauth2/v4/server"
	"github.com/gorilla/mux"
)

type clientStore struct {
}

var _ oauth2.ClientStore = &clientStore{}

// according to the ID for the client information
func (s *clientStore) GetByID(ctx context.Context, id string) (oauth2.ClientInfo, error) {
	return nil, nil
}

type tokenStore struct {
}

var _ oauth2.TokenStore = &tokenStore{}

// create and store the new token information
func (s *tokenStore) Create(ctx context.Context, info oauth2.TokenInfo) error {
	return nil
}

// delete the authorization code
func (s *tokenStore) RemoveByCode(ctx context.Context, code string) error {
	return nil
}

// use the access token to delete the token information
func (s *tokenStore) RemoveByAccess(ctx context.Context, access string) error {
	return nil
}

// use the refresh token to delete the token information
func (s *tokenStore) RemoveByRefresh(ctx context.Context, refresh string) error {
	return nil
}

// use the authorization code for token information data
func (s *tokenStore) GetByCode(ctx context.Context, code string) (oauth2.TokenInfo, error) {
	return nil, nil
}

// use the access token for token information data
func (s *tokenStore) GetByAccess(ctx context.Context, access string) (oauth2.TokenInfo, error) {
	return nil, nil
}

// use the refresh token for token information data
func (s *tokenStore) GetByRefresh(ctx context.Context, refresh string) (oauth2.TokenInfo, error) {
	return nil, nil
}

func NewOAuthProviderHandler() http.Handler {
	router := mux.NewRouter()
	manager := manage.NewDefaultManager()
	manager.MapTokenStorage(&tokenStore{})
	manager.MapClientStorage(&clientStore{})

	srv := server.NewServer(server.NewConfig(), manager)
	srv.SetAllowGetAccessRequest(true)
	srv.SetClientInfoHandler(server.ClientFormHandler)

	srv.UserAuthorizationHandler = func(w http.ResponseWriter, r *http.Request) (userID string, err error) {
		return "000000", nil
	}

	srv.SetInternalErrorHandler(func(err error) *errors.Response {
		// TODO: Maybe log.
		return &errors.Response{
			Error:     err,
			ErrorCode: http.StatusInternalServerError,
		}
	})

	srv.SetResponseErrorHandler(func(re *errors.Response) {
		log.Println("Response Error:", re.Error.Error())
	})

	router.HandleFunc("/authorize", func(w http.ResponseWriter, r *http.Request) {
		err := srv.HandleAuthorizeRequest(w, r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
		}
	})

	router.HandleFunc("/token", func(w http.ResponseWriter, r *http.Request) {
		srv.HandleTokenRequest(w, r)
	})

	return router
}
