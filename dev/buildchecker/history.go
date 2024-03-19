package main

import (
	"sort"
	"strconv"
	"time"

	"github.com/buildkite/go-buildkite/v3/buildkite"
)

func generateHistory(builds []buildkite.Build, windowStart time.Time, opts CheckOptions) (totals map[string]int, flakes map[string]int, incidents map[string]int) {
	// day:count
	totals = make(map[string]int)
	for _, b := range builds {
		totals[buildDate(b.CreatedAt.Time)] += 1
	}
	// day:count
	flakes = make(map[string]int)
	// day:minutes
	incidents = make(map[string]int)

	// Scan over all builds
	scanBuilds := builds
	lastPassedBuild := windowStart
	for len(scanBuilds) > 0 {
		var firstFailedBuildIndex int
		for i, b := range scanBuilds {
			if isBuildFailed(b, opts.BuildTimeout) {
				firstFailedBuildIndex = i
				break
			} else if isBuildPassed(b) {
				lastPassedBuild = b.CreatedAt.Time
			}
		}
		scanBuilds = scanBuilds[max(firstFailedBuildIndex-1, 0):]

		failed, exceeded, scanned := findConsecutiveFailures(
			scanBuilds, opts.FailuresThreshold, opts.BuildTimeout)
		if exceeded {
			// Time from last passed build to oldest build in series
			firstFailed := failed[len(failed)-1]
			redTime := lastPassedBuild.Sub(firstFailed.BuildCreated)
			incidents[buildDate(firstFailed.BuildCreated)] += int(redTime.Minutes())
		} else {
			for _, f := range failed {
				// Raw count of failed builds on date
				flakes[buildDate(f.BuildCreated)] += 1
			}
		}

		if len(scanBuilds) > scanned {
			// Set most recent passed build in last batch
			for _, b := range scanBuilds[:scanned+1] {
				if isBuildPassed(b) {
					lastPassedBuild = b.CreatedAt.Time
				}
			}
			// Scan next batch
			scanBuilds = scanBuilds[scanned+1:]
		} else {
			scanBuilds = []buildkite.Build{}
		}
	}

	return
}

const dateFormat = "2006-01-02"

func buildDate(created time.Time) string {
	return created.Format(dateFormat)
}

func mapToRecords(m map[string]int) (records [][]string) {
	for k, v := range m {
		records = append(records, []string{k, strconv.Itoa(v)})
	}
	// Sort by date ascending
	sort.Slice(records, func(i, j int) bool {
		iDate, _ := time.Parse(dateFormat, records[i][0])
		jDate, _ := time.Parse(dateFormat, records[j][0])
		return iDate.Before(jDate)
	})
	if len(records) <= 1 {
		return
	}
	// Fill in the gaps
	prev := records[0]
	length := len(records)
	for index := range length {
		record := records[index]
		recordDate, _ := time.Parse(dateFormat, record[0])
		prevDate, _ := time.Parse(dateFormat, prev[0])

		for gapDate := prevDate.Add(24 * time.Hour); recordDate.Sub(gapDate) >= 24*time.Hour; gapDate = gapDate.Add(24 * time.Hour) {
			insertRecord := []string{gapDate.Format(dateFormat), "0"}
			records = append(records[:index], append([][]string{insertRecord}, records[index:]...)...)
			index += 1
			length += 1
		}

		prev = record
	}
	return
}
