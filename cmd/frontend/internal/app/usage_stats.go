package app

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/inconshreveable/log15" //nolint:logging // TODO move all logging to sourcegraph/log

	"github.com/sourcegraph/sourcegraph/internal/adminanalytics"
	"github.com/sourcegraph/sourcegraph/internal/auth"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/usagestats"
)

func usageStatsArchiveHandler(db database.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// 🚨SECURITY: Only site admins may get this archive.
		if err := auth.CheckCurrentUserIsSiteAdmin(r.Context(), db); err != nil {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		w.Header().Set("Content-Type", "application/zip")
		w.Header().Set("Content-Disposition", "attachment; filename=\"SourcegraphUsersUsageArchive.zip\"")

		archive, err := usagestats.GetArchive(r.Context(), db)
		if err != nil {
			log15.Error("usagestats.WriteArchive", "error", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		_, _ = w.Write(archive)
	}
}

func allEventsArchiveHandler(db database.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// 🚨SECURITY: Only site admins may get this archive.
		if err := auth.CheckCurrentUserIsSiteAdmin(r.Context(), db); err != nil {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		namesQuery := r.URL.Query().Get("names")
		if len(namesQuery) == 0 {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		names := strings.Split(namesQuery, ",")

		dateRangeQuery := r.URL.Query().Get("dateRange")
		if len(dateRangeQuery) == 0 {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		w.Header().Set("Content-Type", "application/zip")
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"SourcegraphEventLogs-%s.zip\"", time.Now().Format(time.RFC3339)))

		archive, err := adminanalytics.GetAllEventsArchive(r.Context(), db, names, dateRangeQuery)
		if err != nil {
			log15.Error("adminanalytics.GetAllEventsArchive", "error", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		_, _ = w.Write(archive)
	}
}
