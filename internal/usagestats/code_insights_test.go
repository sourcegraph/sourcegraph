package usagestats

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"github.com/sourcegraph/sourcegraph/internal/database/dbtesting"
	"github.com/sourcegraph/sourcegraph/internal/types"
)

func TestCodeInsightsUsageStatistics(t *testing.T) {
	ctx := context.Background()

	defer func() {
		timeNow = time.Now
	}()

	weekStart := time.Date(2021, 1, 25, 0, 0, 0, 0, time.UTC)
	now := time.Date(2021, 1, 28, 0, 0, 0, 0, time.UTC)
	mockTimeNow(now)

	db := dbtesting.GetDB(t)

	_, err := db.Exec(`
		INSERT INTO event_logs
			(id, name, argument, url, user_id, anonymous_user_id, source, version, timestamp)
		VALUES
			(1, 'ViewInsights', '{}', '', 1, '420657f0-d443-4d16-ac7d-003d8cdc91ef', 'WEB', '3.23.0', $1::timestamp - interval '1 day'),
			(2, 'ViewInsights', '{}', '', 1, '420657f0-d443-4d16-ac7d-003d8cdc91ef', 'WEB', '3.23.0', $1::timestamp - interval '1 day'),
			(3, 'InsightAddition', '{"insightType": "searchInsights"}', '', 1, '420657f0-d443-4d16-ac7d-003d8cdc91ef', 'WEB', '3.23.0', $1::timestamp - interval '1 day'),
			(4, 'InsightAddition', '{"insightType": "codeStatsInsights"}', '', 2, '420657f0-d443-4d16-ac7d-003d8cdc19ac', 'WEB', '3.23.0', $1::timestamp - interval '1 day'),
			(5, 'InsightAddition', '{"insightType": "searchInsights"}', '', 2, '420657f0-d443-4d16-ac7d-003d8cdc19ac', 'WEB', '3.23.0', $1::timestamp - interval '1 day'),
			(6, 'InsightEdit', '{"insightType": "searchInsights"}', '', 2, '420657f0-d443-4d16-ac7d-003d8cdc19ac', 'WEB', '3.23.0', $1::timestamp - interval '2 days'),
			(7, 'InsightAddition', '{"insightType": "codeStatsInsights"}', '', 1, '420657f0-d443-4d16-ac7d-003d8cdc91ef', 'WEB', '3.23.0', $1::timestamp - interval '8 days'),
			(8, 'CodeInsightsSearchBasedCreationPageSubmitClick', '{}', '', 1, '420657f0-d443-4d16-ac7d-003d8cdc91ef', 'WEB', '3.23.0', $1::timestamp - interval '1 day')

	`, now)
	if err != nil {
		t.Fatal(err)
	}

	have, err := GetCodeInsightsUsageStatistics(ctx, db)
	if err != nil {
		t.Fatal(err)
	}

	zeroInt := int32(0)
	oneInt := int32(1)
	twoInt := int32(2)

	searchInsightsType := "searchInsights"
	codeStatsInsightsType := "codeStatsInsights"

	weeklyUsageStatisticsByInsight := []*types.InsightUsageStatistics{
		{
			InsightType:      &codeStatsInsightsType,
			Additions:        &oneInt,
			Edits:            &zeroInt,
			Removals:         &zeroInt,
			Hovers:           &zeroInt,
			UICustomizations: &zeroInt,
			DataPointClicks:  &zeroInt,
		},
		{
			InsightType:      &searchInsightsType,
			Additions:        &twoInt,
			Edits:            &oneInt,
			Removals:         &zeroInt,
			Hovers:           &zeroInt,
			UICustomizations: &zeroInt,
			DataPointClicks:  &zeroInt,
		},
	}

	want := &types.CodeInsightsUsageStatistics{
		WeeklyUsageStatisticsByInsight: weeklyUsageStatisticsByInsight,
		WeeklyInsightsPageViews:        &twoInt,
		WeeklyInsightsUniquePageViews:  &oneInt,
		WeeklyInsightConfigureClick:    &zeroInt,
		WeeklyInsightAddMoreClick:      &zeroInt,
		WeekStart:                      weekStart,
		WeeklyInsightCreators:          &twoInt,
		WeeklyFirstTimeInsightCreators: &oneInt,
	}

	wantedWeeklyUsage := []types.AggregatedPingStats{
		{Name: "CodeInsightsSearchBasedCreationPageSubmitClick", TotalCount: 1, UniqueCount: 1},
	}

	want.WeeklyAggregatedUsage = wantedWeeklyUsage
	want.InsightTimeIntervals = []types.InsightTimeIntervalPing{}
	want.InsightOrgVisible = []types.OrgVisibleInsightPing{}

	if diff := cmp.Diff(want, have); diff != "" {
		t.Fatal(diff)
	}
}

func TestWithCreationPings(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2021, 1, 28, 0, 0, 0, 0, time.UTC)

	db := dbtesting.GetDB(t)

	user1 := "420657f0-d443-4d16-ac7d-003d8cdc91ef"
	user2 := "55555555-5555-5555-5555-555555555555"

	_, err := db.Exec(`
		INSERT INTO event_logs
			(id, name, argument, url, user_id, anonymous_user_id, source, version, timestamp)
		VALUES
			(1, 'ViewInsights', '{}', '', 1, $2, 'WEB', '3.23.0', $1::timestamp - interval '1 day'),
			(2, 'ViewInsights', '{}', '', 1, $2, 'WEB', '3.23.0', $1::timestamp - interval '1 day'),
			(3, 'ViewCodeInsightsCreationPage', '{}', '', 1, $2, 'WEB', '3.23.0', $1::timestamp - interval '1 day'),
			(4, 'ViewCodeInsightsCreationPage', '{}', '', 1, $2, 'WEB', '3.23.0', $1::timestamp - interval '10 days'),
			(5, 'CodeInsightsExploreInsightExtensionsClick', '{}', '', 2, $3, 'WEB', '3.23.0', $1::timestamp - interval '1 day'),
			(6, 'CodeInsightsExploreInsightExtensionsClick', '{}', '', 2, $3, 'WEB', '3.23.0', $1::timestamp - interval '10 days'),
			(7, 'ViewCodeInsightsCreationPage', '{}', '', 2, $3, 'WEB', '3.23.0', $1::timestamp - interval '2 days'),
			(8, 'ViewCodeInsightsCreationPage', '{}', '', 2, $3, 'WEB', '3.23.0', $1::timestamp - interval '2 days')
	`, now, user1, user2)
	if err != nil {
		t.Fatal(err)
	}

	want := map[types.PingName]types.AggregatedPingStats{
		"CodeInsightsExploreInsightExtensionsClick": {Name: "CodeInsightsExploreInsightExtensionsClick", UniqueCount: 1, TotalCount: 1},
		"ViewCodeInsightsCreationPage":              {Name: "ViewCodeInsightsCreationPage", UniqueCount: 2, TotalCount: 3},
	}

	results, err := GetCreationViewUsage(ctx, db, func() time.Time {
		return now
	})
	if err != nil {
		t.Fatal(err)
	}

	// convert into map so we can reliably test for equality
	got := make(map[types.PingName]types.AggregatedPingStats)
	for _, v := range results {
		got[v.Name] = v
	}

	if !cmp.Equal(want, got) {
		t.Fatal(fmt.Sprintf("want: %v got: %v", want, got))
	}
}

func TestGetInsightTimeIntervals(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2021, 1, 28, 0, 0, 0, 0, time.UTC)

	db := dbtesting.GetDB(t)

	user1 := "420657f0-d443-4d16-ac7d-003d8cdc91ef"

	_, err := db.Exec(`
		INSERT INTO event_logs
			(id, name, argument, url, user_id, anonymous_user_id, source, version, timestamp)
		VALUES
			(1, 'InsightsGroupedStepSizes', '[60, 90, 30]', '', 1, $2, 'WEB', '3.23.0', $1::timestamp - interval '2 day'),
			(2, 'InsightsGroupedStepSizes', '[60, 90, 30, 60]', '', 1, $2, 'WEB', '3.23.0', $1::timestamp - interval '1 day'),
			(3, 'NotIncluded', '[60, 90, 30]', '', 1, $2, 'WEB', '3.23.0', $1::timestamp - interval '1 day')
	`, now, user1)
	if err != nil {
		t.Fatal(err)
	}

	want := []types.InsightTimeIntervalPing{
		{
			IntervalDays: 30,
			TotalCount:   1,
		},
		{
			IntervalDays: 60,
			TotalCount:   2,
		},
		{
			IntervalDays: 90,
			TotalCount:   1,
		},
	}

	got, err := GetInsightTimeIntervals(ctx, db, func() time.Time {
		return now
	})

	if err != nil {
		t.Fatal(err)
	}

	if !cmp.Equal(want, got) {
		t.Fatal(fmt.Sprintf("want: %v got: %v", want, got))
	}
}

func TestGetInsightCountsByOrg(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2021, 1, 28, 0, 0, 0, 0, time.UTC)

	db := dbtesting.GetDB(t)

	user1 := "420657f0-d443-4d16-ac7d-003d8cdc91ef"

	_, err := db.Exec(`
		INSERT INTO event_logs
			(id, name, argument, url, user_id, anonymous_user_id, source, version, timestamp)
		VALUES
			(1, 'InsightsGroupedCount', '{"codeStatsInsights": 0, "searchBasedInsights": 2}', '', 1, $2, 'WEB', '3.23.0', $1::timestamp - interval '2 day'),
			(2, 'InsightsGroupedCount', '{"codeStatsInsights": 1, "searchBasedInsights": 3}', '', 1, $2, 'WEB', '3.23.0', $1::timestamp - interval '1 day'),
			(3, 'NotIncluded', '[60, 90, 30]', '', 1, $2, 'WEB', '3.23.0', $1::timestamp - interval '1 day')
	`, now, user1)
	if err != nil {
		t.Fatal(err)
	}

	want := []types.OrgVisibleInsightPing{
		{
			Type:       "codeStatsInsights",
			TotalCount: 1,
		},
		{
			Type:       "searchBasedInsights",
			TotalCount: 3,
		},
	}

	got, err := GetInsightCountsByOrg(ctx, db, func() time.Time {
		return now
	})

	if err != nil {
		t.Fatal(err)
	}

	if !cmp.Equal(want, got) {
		t.Fatal(fmt.Sprintf("want: %v got: %v", want, got))
	}
}
