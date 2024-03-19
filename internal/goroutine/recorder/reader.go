package recorder

import (
	"context"
	"encoding/json"
	"sort"
	"time"

	"github.com/sourcegraph/sourcegraph/internal/rcache"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

// GetBackgroundJobInfos returns information about all known jobs.
func GetBackgroundJobInfos(c *rcache.Cache, after string, recentRunCount int, dayCountForStats int) ([]JobInfo, error) {
	// Get known job names sorted by name, ascending
	knownJobNames, err := getKnownJobNames(c)
	if err != nil {
		return nil, errors.Wrap(err, "get known job names")
	}

	// Get all jobs
	jobs := make([]JobInfo, 0, len(knownJobNames))
	for _, jobName := range knownJobNames {
		job, err := GetBackgroundJobInfo(c, jobName, recentRunCount, dayCountForStats)
		if err != nil {
			return nil, errors.Wrapf(err, "get job info for %q", jobName)
		}
		jobs = append(jobs, job)
	}

	// Filter jobs by name to respect "after" (they are ordered by name)
	if after != "" {
		for i, job := range jobs {
			if job.Name > after {
				return jobs[i:], nil
			}
		}
	}

	return jobs, nil
}

// GetBackgroundJobInfo returns information about the given job.
func GetBackgroundJobInfo(c *rcache.Cache, jobName string, recentRunCount int, dayCountForStats int) (JobInfo, error) {
	allHostNames, err := getKnownHostNames(c)
	if err != nil {
		return JobInfo{}, err
	}

	routines, err := getKnownRoutinesForJob(c, jobName)
	if err != nil {
		return JobInfo{}, err
	}

	routineInfos := make([]RoutineInfo, 0, len(routines))
	for _, r := range routines {
		routineInfo, err := getRoutineInfo(c, r, allHostNames, recentRunCount, dayCountForStats)
		if err != nil {
			return JobInfo{}, err
		}

		routineInfos = append(routineInfos, routineInfo)
	}

	return JobInfo{ID: jobName, Name: jobName, Routines: routineInfos}, nil
}

// getKnownJobNames returns a list of all known job names, ascending, filtered by their “last seen” time.
func getKnownJobNames(c *rcache.Cache) ([]string, error) {
	jobNames, err := c.GetHashAll("knownJobNames")
	if err != nil {
		return nil, err
	}

	// Get the values only from the map
	var values []string
	for jobName, lastSeenString := range jobNames {
		// Parse “last seen” time
		lastSeen, err := time.Parse(time.RFC3339, lastSeenString)
		if err != nil {
			return nil, errors.Wrap(err, "failed to parse job last seen time")
		}

		// Check if job is still running
		if time.Since(lastSeen) > seenTimeout {
			continue
		}

		values = append(values, jobName)
	}

	// Sort the values
	sort.Strings(values)

	return values, nil
}

// getKnownHostNames returns a list of all known host names, ascending, filtered by their “last seen” time.
func getKnownHostNames(c *rcache.Cache) ([]string, error) {
	hostNames, err := c.GetHashAll("knownHostNames")
	if err != nil {
		return nil, err
	}

	// Get the values only from the map
	var values []string
	for hostName, lastSeenString := range hostNames {
		// Parse “last seen” time
		lastSeen, err := time.Parse(time.RFC3339, lastSeenString)
		if err != nil {
			return nil, errors.Wrap(err, "failed to parse host last seen time")
		}

		// Check if job is still running
		if time.Since(lastSeen) > seenTimeout {
			continue
		}

		values = append(values, hostName)
	}

	// Sort the values
	sort.Strings(values)

	return values, nil
}

// getKnownRoutinesForJob returns a list of all known recordables for the given job name, ascending.
func getKnownRoutinesForJob(c *rcache.Cache, jobName string) ([]serializableRoutineInfo, error) {
	// Get all recordables
	routines, err := getKnownRoutines(c)
	if err != nil {
		return nil, err
	}

	// Filter by job name
	var routinesForJob []serializableRoutineInfo
	for _, r := range routines {
		if r.JobName == jobName {
			routinesForJob = append(routinesForJob, r)
		}
	}

	// Sort them by name
	sort.Slice(routinesForJob, func(i, j int) bool {
		return routinesForJob[i].Name < routinesForJob[j].Name
	})

	return routinesForJob, nil
}

// getKnownRoutines returns a list of all known recordables, unfiltered, in no particular order.
func getKnownRoutines(c *rcache.Cache) ([]serializableRoutineInfo, error) {
	rawItems, err := c.GetHashAll("knownRoutines")
	if err != nil {
		return nil, err
	}

	routines := make([]serializableRoutineInfo, 0, len(rawItems))
	for _, rawItem := range rawItems {
		var item serializableRoutineInfo
		err := json.Unmarshal([]byte(rawItem), &item)
		if err != nil {
			return nil, err
		}
		routines = append(routines, item)
	}
	return routines, nil
}

// getRoutineInfo returns the info for a single routine: its instances, recent runs, and stats.
func getRoutineInfo(c *rcache.Cache, r serializableRoutineInfo, allHostNames []string, recentRunCount int, dayCountForStats int) (RoutineInfo, error) {
	routineInfo := RoutineInfo{
		Name:        r.Name,
		Type:        r.Type,
		JobName:     r.JobName,
		Description: r.Description,
		IntervalMs:  int32(r.Interval / time.Millisecond),
		Instances:   make([]RoutineInstanceInfo, 0, len(allHostNames)),
		RecentRuns:  []RoutineRun{},
	}

	// Collect instances
	for _, hostName := range allHostNames {
		instanceInfo, err := getRoutineInstanceInfo(c, r.JobName, r.Name, hostName)
		if err != nil {
			return RoutineInfo{}, err
		}

		routineInfo.Instances = append(routineInfo.Instances, instanceInfo)
	}

	// Collect recent runs
	for _, hostName := range allHostNames {
		recentRunsForHost, err := loadRecentRuns(c, r.JobName, r.Name, hostName, recentRunCount)
		if err != nil {
			return RoutineInfo{}, err
		}

		routineInfo.RecentRuns = append(routineInfo.RecentRuns, recentRunsForHost...)
	}

	// Sort recent runs descending by start time
	sort.Slice(routineInfo.RecentRuns, func(i, j int) bool {
		return routineInfo.RecentRuns[i].At.After(routineInfo.RecentRuns[j].At)
	})
	// Limit to recentRunCount
	if len(routineInfo.RecentRuns) > recentRunCount {
		routineInfo.RecentRuns = routineInfo.RecentRuns[:recentRunCount]
	}

	// Collect stats
	stats, err := loadRunStats(c, r.JobName, r.Name, time.Now(), dayCountForStats)
	if err != nil {
		return RoutineInfo{}, errors.Wrap(err, "load run stats")
	}
	routineInfo.Stats = stats

	return routineInfo, nil
}

// getRoutineInstanceInfo returns the info for a single routine instance.
func getRoutineInstanceInfo(c *rcache.Cache, jobName string, routineName string, hostName string) (RoutineInstanceInfo, error) {
	var lastStart *time.Time
	var lastStop *time.Time

	lastStartBytes, ok := c.Get(jobName + ":" + routineName + ":" + hostName + ":" + "lastStart")
	if ok {
		t, err := time.Parse(time.RFC3339, string(lastStartBytes))
		if err != nil {
			return RoutineInstanceInfo{}, errors.Wrap(err, "parse last start")
		}
		lastStart = &t
	}

	lastStopBytes, ok := c.Get(jobName + ":" + routineName + ":" + hostName + ":" + "lastStop")
	if ok {
		t, err := time.Parse(time.RFC3339, string(lastStopBytes))
		if err != nil {
			return RoutineInstanceInfo{}, errors.Wrap(err, "parse last stop")
		}
		lastStop = &t
	}

	return RoutineInstanceInfo{
		HostName:      hostName,
		LastStartedAt: lastStart,
		LastStoppedAt: lastStop,
	}, nil
}

// loadRecentRuns loads the recent runs for a routine, in no particular order.
func loadRecentRuns(c *rcache.Cache, jobName string, routineName string, hostName string, count int) ([]RoutineRun, error) {
	recentRuns, err := getRecentRuns(c, jobName, routineName, hostName).Slice(context.Background(), 0, count)
	if err != nil {
		return nil, errors.Wrap(err, "load recent runs")
	}

	runs := make([]RoutineRun, 0, len(recentRuns))
	for _, serializedRun := range recentRuns {
		var run RoutineRun
		err := json.Unmarshal(serializedRun, &run)
		if err != nil {
			return nil, errors.Wrap(err, "deserialize run")
		}
		runs = append(runs, run)
	}

	return runs, nil
}

// loadRunStats loads the run stats for a routine.
func loadRunStats(c *rcache.Cache, jobName string, routineName string, now time.Time, dayCount int) (RoutineRunStats, error) {
	// Get all stats
	var stats RoutineRunStats
	for i := range dayCount {
		date := now.AddDate(0, 0, -i).Truncate(24 * time.Hour)
		statsRaw, found := c.Get(jobName + ":" + routineName + ":runStats:" + date.Format("2006-01-02"))
		if found {
			var statsForDay RoutineRunStats
			err := json.Unmarshal(statsRaw, &statsForDay)
			if err != nil {
				return RoutineRunStats{}, errors.Wrap(err, "deserialize stats for day")
			}
			mergedStats := mergeStats(stats, statsForDay)

			// Temporary code: There was a bug that messed up past averages.
			// This block helps ignore that messed-up data.
			// We can pretty safely remove this in four months.
			if mergedStats.AvgDurationMs < 0 {
				mergedStats.AvgDurationMs = stats.AvgDurationMs
			}

			stats = mergedStats
			if stats.Since.IsZero() {
				stats.Since = date
			}
		}
	}

	return stats, nil
}
