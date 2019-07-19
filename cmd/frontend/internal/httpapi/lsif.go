package httpapi

import (
	"crypto/hmac"
	"crypto/sha256"
	"strings"

	"encoding/hex"
	"encoding/json"

	"fmt"

	"net/http"
	"net/http/httputil"
	"net/url"

	"github.com/gorilla/mux"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/backend"
	"github.com/sourcegraph/sourcegraph/pkg/actor"
	"github.com/sourcegraph/sourcegraph/pkg/api"
	"github.com/sourcegraph/sourcegraph/pkg/env"
	"github.com/sourcegraph/sourcegraph/pkg/extsvc/github"

	"strconv"
)

var verificationGitHubToken = env.Get("LSIF_VERIFICATION_GITHUB_TOKEN", "", "The GitHub token that is used to verify that a user owns a repository.")
var lsifUploadSecret = env.Get("LSIF_UPLOAD_SECRET", "", "Used to generate LSIF upload tokens. Must be long (160+ bits) to make offline brute-force attacks difficult.")
var HasLSIFUploadSecret = lsifUploadSecret != ""

func lsifProxyHandler(p *httputil.ReverseProxy) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		r.URL.Path = mux.Vars(r)["rest"]
		p.ServeHTTP(w, r)
	}
}

func generateUploadToken(repoID api.RepoID) []byte {
	mac := hmac.New(sha256.New, []byte(lsifUploadSecret))
	mac.Write([]byte(strconv.Itoa(int(repoID))))
	return mac.Sum(nil)
}

func generateChallenge(userID int32) string {
	// Must be different from the upload token and different for each user
	sum := sha256.Sum256([]byte(fmt.Sprintf("%d:%s", userID, lsifUploadSecret)))
	// The first 10 hex digits is enough to be fairly confident that a GitHub
	// topic of this name doesn't already exist on the repository.
	return hex.EncodeToString(sum[:])[:10]
}

// isValidUploadToken checks whether token is a valid upload token for repoID.
func isValidUploadToken(repoID api.RepoID, token string) bool {
	gotMAC, err := hex.DecodeString(token)
	if err != nil {
		return false
	}

	return hmac.Equal(gotMAC, generateUploadToken(repoID))
}

func lsifChallengeHandler(w http.ResponseWriter, r *http.Request) {
	actor := actor.FromContext(r.Context())
	jData, err := json.Marshal(struct{ Challenge string }{Challenge: generateChallenge(actor.UID)})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(jData)
}

func lsifVerifyHandler(w http.ResponseWriter, r *http.Request) {
	repositories, ok := r.URL.Query()["repository"]
	if !ok || len(repositories) == 0 {
		http.Error(w, "No repository was specified.", http.StatusBadRequest)
		return
	}
	repository := repositories[0]

	if !strings.HasPrefix(repository, "github.com") {
		http.Error(w, "Only github.com repositories support verification.", http.StatusUnprocessableEntity)
		return
	}
	ownerAndName := strings.TrimPrefix(repository, "github.com")

	repo, err := backend.Repos.GetByName(r.Context(), api.RepoName(repository))
	if err != nil {
		http.Error(w, "Unknown repository.", http.StatusUnauthorized)
		return
	}

	actor := actor.FromContext(r.Context())

	apiURL, err := url.Parse("https://api.github.com")
	if err != nil {
		http.Error(w, "Error parsing API URL.", http.StatusInternalServerError)
		return
	}
	client := github.NewClient(apiURL, verificationGitHubToken, nil)
	topics, err := client.ListTopicsOnRepository(r.Context(), ownerAndName)
	if err != nil {
		fmt.Println(err)
		http.Error(w, "Error listing topics.", http.StatusInternalServerError)
		return
	}
	set := make(map[string]bool)
	for _, v := range topics {
		set[v] = true
	}
	success := set[generateChallenge(actor.UID)]

	var payload interface{}
	if success {
		token := generateUploadToken(repo.ID)
		payload = struct{ Token string }{Token: hex.EncodeToString(token[:])}
	} else {
		payload = struct{ Failure string }{Failure: "Topic not found."}
	}
	jData, err := json.Marshal(payload)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(jData)
}

func lsifUploadProxyHandler(p *httputil.ReverseProxy) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		tokens, ok := r.URL.Query()["upload_token"]
		if !ok || len(tokens) == 0 {
			http.Error(w, "No LSIF upload token was given.", http.StatusBadRequest)
			return
		}
		givenToken := tokens[0]

		repositories, ok := r.URL.Query()["repository"]
		if !ok || len(repositories) == 0 {
			http.Error(w, "No repository was specified.", http.StatusBadRequest)
			return
		}
		repository := repositories[0]

		repo, err := backend.Repos.GetByName(r.Context(), api.RepoName(repository))
		if err != nil {
			http.Error(w, "Unknown repository.", http.StatusUnauthorized)
			return
		}

		if !isValidUploadToken(repo.ID, givenToken) {
			http.Error(w, "Invalid LSIF upload token.", http.StatusUnauthorized)
			return
		}

		r.URL.Path = "upload"
		p.ServeHTTP(w, r)
	}
}
