package httpapi

import (
	"fmt"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/pkg/errors"

	"github.com/sourcegraph/sourcegraph/cmd/frontend/backend"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/internal/routevar"
)

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

func serveRepoShield(w http.ResponseWriter, r *http.Request) error {
	value, err := badgeValue(r)
	if err != nil {
		return err
	}
	return writeJSON(w, &struct {
		// Note: Named lowercase because the JSON is consumed by shields.io JS
		// code.
		Value string `json:"value"`
	}{
		Value: badgeValueFmt(value),
	})
}
