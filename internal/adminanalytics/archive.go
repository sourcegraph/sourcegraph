package adminanalytics

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/csv"
	"strconv"
	"strings"
	"time"

	"github.com/sourcegraph/sourcegraph/internal/database"
)

// GetAllEventsArchive generates and returns an statistics ZIP archive containing a CSV
// file containing all user events that match an event name query.
func GetAllEventsArchive(ctx context.Context, db database.DB, eventNames []string, dateRange string) ([]byte, error) {
	fromDate, err := getFromDate(dateRange, time.Now())
	if err != nil {
		return nil, err
	}

	// trim space from all event names
	trimmedEventNames := make([]string, 0, len(eventNames))
	for _, name := range eventNames {
		trimmedEventNames = append(trimmedEventNames, strings.TrimSpace(name))
	}

	events, err := db.EventLogs().AllEvents(ctx, trimmedEventNames, fromDate, time.Now())
	if err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)

	eventsFile, err := zw.Create("SourcegraphEventLogs.csv")
	if err != nil {
		return nil, err
	}

	eventsWriter := csv.NewWriter(eventsFile)

	record := []string{
		"id",
		"name",
		"url",
		"user_id",
		"anonymous_device_user_id",
		"client",
		"source",
		"timestamp",
		"event_metadata",
	}

	if err := eventsWriter.Write(record); err != nil {
		return nil, err
	}

	for _, e := range events {
		record[0] = strconv.FormatUint(uint64(e.ID), 10)
		record[1] = e.Name
		record[2] = e.URL
		record[3] = strconv.FormatUint(uint64(e.UserID), 10)
		record[4] = e.AnonymousUserID
		if e.Client != nil {
			record[5] = *e.Client
		}
		record[6] = e.Source
		record[7] = e.Timestamp.UTC().Format(time.RFC3339)
		record[8] = e.Argument

		if err := eventsWriter.Write(record); err != nil {
			return nil, err
		}
	}

	eventsWriter.Flush()

	if err := zw.Close(); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}
