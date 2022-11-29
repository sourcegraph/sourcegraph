package insights

import (
	"sort"
	"time"
)

func calculateRecordingTimes(createdAt time.Time, lastRecordedAt time.Time, interval timeInterval, existingPoints []time.Time) []time.Time {
	referenceTimes := buildRecordingTimes(12, interval, createdAt.Truncate(time.Hour*24))
	if !lastRecordedAt.IsZero() {
		// If we've had recordings since we need to step through them.
		referenceTimes = append(referenceTimes, buildRecordingTimesBetween(createdAt.Truncate(time.Hour*24), lastRecordedAt, interval)[1:]...)
	}

	if len(existingPoints) == 0 {
		return referenceTimes
	}

	// The set of recording times will be augmented with zeros for missing points in the expected leading set and the
	// expected trailing set.

	var calculatedRecordingTimes []time.Time

	// If the first existing point is newer than the oldest expected point then leading points are added.
	oldestReferencePoint := referenceTimes[0]
	if !withinHalfAnInterval(existingPoints[0], oldestReferencePoint, interval) {
		referencePoint, index := oldestReferencePoint, 0
		for referencePoint.Before(existingPoints[index]) {
			calculatedRecordingTimes = append(calculatedRecordingTimes, referencePoint)
			index++
			referencePoint = referenceTimes[index]
		}
	}
	// Any existing middle points are added.
	calculatedRecordingTimes = append(calculatedRecordingTimes, existingPoints...)

	// If the last existing point is older than the newest expected point then trailing points are added.
	newestReferencePoint := referenceTimes[len(referenceTimes)-1]
	if !withinHalfAnInterval(newestReferencePoint, existingPoints[len(existingPoints)-1], interval) {
		referencePoint, i := newestReferencePoint, len(existingPoints)-1
		var backwardTrailingPoints []time.Time
		for existingPoints[i].Before(referencePoint) {
			backwardTrailingPoints = append(backwardTrailingPoints, referencePoint)
			i--
			referencePoint = referenceTimes[i]
		}
		for i := len(backwardTrailingPoints) - 1; i >= 0; i-- {
			calculatedRecordingTimes = append(calculatedRecordingTimes, backwardTrailingPoints[i])
		}
	}

	return calculatedRecordingTimes
}

func withinHalfAnInterval(firstTime, secondTime time.Time, interval timeInterval) bool {
	intervalDuration := interval.toDuration() // precise to rough estimate of an interval's length (e.g. 1 year = 365 * 24 hours)
	halfAnInterval := intervalDuration / 2
	if interval.unit == hour {
		halfAnInterval = intervalDuration / 4
	}
	differenceInExpectedTime := firstTime.Sub(secondTime)
	return differenceInExpectedTime >= 0 && differenceInExpectedTime <= halfAnInterval
}

type intervalUnit string

const (
	month intervalUnit = "MONTH"
	day   intervalUnit = "DAY"
	week  intervalUnit = "WEEK"
	year  intervalUnit = "YEAR"
	hour  intervalUnit = "HOUR"
)

type timeInterval struct {
	unit  intervalUnit
	value int
}

func (t timeInterval) toDuration() time.Duration {
	var singleUnitDuration time.Duration
	switch t.unit {
	case year:
		singleUnitDuration = time.Hour * 24 * 365
	case month:
		singleUnitDuration = time.Hour * 24 * 30
	case week:
		singleUnitDuration = time.Hour * 24 * 7
	case day:
		singleUnitDuration = time.Hour * 24
	case hour:
		singleUnitDuration = time.Hour
	}
	return singleUnitDuration * time.Duration(t.value)
}

func buildRecordingTimes(numPoints int, interval timeInterval, now time.Time) []time.Time {
	current := now
	times := make([]time.Time, 0, numPoints)
	times = append(times, now)

	for i := 0 - numPoints + 1; i < 0; i++ {
		current = interval.stepBackwards(current)
		times = append(times, current)
	}

	sort.Slice(times, func(i, j int) bool {
		return times[i].Before(times[j])
	})
	return times
}

// buildRecordingTimesBetween builds times starting at `start` up until `end` at the given interval.
func buildRecordingTimesBetween(start time.Time, end time.Time, interval timeInterval) []time.Time {
	times := []time.Time{}

	current := start
	for current.Before(end) {
		times = append(times, current)
		current = interval.stepForwards(current)
	}

	return times
}

func (t timeInterval) stepBackwards(start time.Time) time.Time {
	return t.step(start, backward)
}

func (t timeInterval) stepForwards(start time.Time) time.Time {
	return t.step(start, forward)
}

type stepDirection int

const (
	forward  stepDirection = 1
	backward stepDirection = -1
)

func (t timeInterval) step(start time.Time, direction stepDirection) time.Time {
	switch t.unit {
	case year:
		return start.AddDate(int(direction)*t.value, 0, 0)
	case month:
		return start.AddDate(0, int(direction)*t.value, 0)
	case week:
		return start.AddDate(0, 0, int(direction)*7*t.value)
	case day:
		return start.AddDate(0, 0, int(direction)*t.value)
	case hour:
		return start.Add(time.Hour * time.Duration(t.value) * time.Duration(direction))
	default:
		// this doesn't really make sense, so return something?
		return start.AddDate(int(direction)*t.value, 0, 0)
	}
}
