package app

import (
	"fmt"
	"net/http"
	"net/url"

	"github.com/pkg/errors"

	"github.com/gorilla/mux"

	"github.com/sourcegraph/sourcegraph/cmd/frontend/backend"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/internal/routevar"
)

// TODO(slimsag): once https://github.com/badges/shields/pull/828 is merged,
// redirect to our more canonical shields.io URLs and remove this badgeValue
// duplication kludge.

// NOTE: Keep in sync with services/backend/httpapi/repo_shield.go
func badgeValue(r *http.Request) (int, error) {
	totalRefs, err := backend.CountGoImporters(r.Context(), routevar.ToRepo(mux.Vars(r)))
	if err != nil {
		return 0, errors.Wrap(err, "Defs.TotalRefs")
	}
	return totalRefs, nil
}

// NOTE: Keep in sync with services/backend/httpapi/repo_shield.go
func badgeValueFmt(totalRefs int) string {
	// Format e.g. "1,399" as "1.3k".
	desc := fmt.Sprintf("%d projects", totalRefs)
	if totalRefs >= 1000 {
		desc = fmt.Sprintf("%.1fk projects", float64(totalRefs)/1000.0)
	}

	// Note: We're adding a prefixed space because otherwise the shields.io
	// badge will be formatted badly (looks like `used by |12k projects`
	// instead of `used by | 12k projects`).
	return " " + desc
}

func serveRepoBadge(w http.ResponseWriter, r *http.Request) error {
	value, err := badgeValue(r)
	if err != nil {
		return err
	}

	v := url.Values{}
	v.Set("logo", "data:image/svg+xml;base64,PHN2ZyB4bWxucz0iaHR0cDovL3d3dy53My5vcmcvMjAwMC9zdmciIHZpZXdCb3g9IjAgMCA3Ny4wNzUgNzcuNyI+PHBhdGggZmlsbD0iI0ZGRiIgZD0iTTQ3LjMyMyA3Ny43Yy0zLjU5NCAwLTYuNzktMi4zOTYtNy43ODctNS45OWwtMTcuMTcyLTYxLjdjLS45OTgtNC4zOTMgMS41OTgtOC43ODYgNS45OS05Ljc4NCA0LjE5My0xIDguMzg3IDEuMzk3IDkuNTg0IDUuMzlsMTYuOTczIDYxLjdjMS4xOTggNC4zOTQtMS4zOTcgOC43ODYtNS41OSA5Ljk4NC0uNi4yLTEuMzk3LjQtMS45OTcuNHoiLz48cGF0aCBmaWxsPSIjRkZGIiBkPSJNMTcuMzcyIDcwLjcxYy00LjM5MyAwLTcuOTg3LTMuNTkzLTcuOTg3LTcuOTg1IDAtMS45OTcuOC0zLjk5NCAxLjk5Ny01LjM5Mkw1NC4xMTIgOS40MWMyLjk5NS0zLjM5MyA3Ljk4Ni0zLjU5MyAxMS4zOC0uNTk4czMuNTk1IDcuOTg3LjYgMTEuMzhsLTQyLjczIDQ3LjcyM2MtMS41OTcgMS43OTgtMy43OTQgMi43OTYtNS45OSAyLjc5NnoiLz48cGF0aCBmaWxsPSIjRkZGIiBkPSJNNjkuMDg3IDU2LjczNGMtLjc5OCAwLTEuNTk3LS4yLTIuNTk2LS40TDUuNTkgMzYuMzY4QzEuNCAzNC45Ny0uOTk3IDMwLjM3Ny40IDI2LjE4NGMxLjM5Ny00LjE5MyA1Ljk5LTYuNTkgMTAuMTgzLTUuMTlsNjAuOSAxOS45NjZjNC4xOTMgMS4zOTcgNi41OSA1Ljk5IDUuMTkgMTAuMTg0LS45OTYgMy4zOTQtMy45OSA1LjU5LTcuNTg2IDUuNTl6Ii8+PC9zdmc+")

	// Allow users to pick the style of badge.
	if val := r.URL.Query().Get("style"); val != "" {
		v.Set("style", val)
	}

	u := &url.URL{
		Scheme:   "https",
		Host:     "img.shields.io",
		Path:     "/badge/used by-" + badgeValueFmt(value) + "-brightgreen.svg",
		RawQuery: v.Encode(),
	}
	http.Redirect(w, r, u.String(), http.StatusTemporaryRedirect)
	return nil
}
