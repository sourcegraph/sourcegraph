package main

import (
	"context"

	"cloud.google.com/go/bigquery"

	"github.com/sourcegraph/sourcegraph/lib/errors"
)

func runQuery(ctx context.Context, bq bigquery.Client, queryStr string, params []bigquery.QueryParameter) (*bigquery.RowIterator, error) {
	query := bq.Query(queryStr)
	query.Parameters = params

	job, err := query.Run(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to start query")
	}
	status, err := job.Wait(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to wait for query")
	}
	if err := status.Err(); err != nil {
		return nil, errors.Wrap(err, "query failed to complete")
	}
	it, err := job.Read(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to read query results")
	}

	return it, nil
}

const topNSumTime = `WITH extracted_owner AS (
	SELECT
		*,
		REGEXP_EXTRACT(bt.tags, 'owner_\\w+') AS owner
	FROM buildkite_analytics.bazel_tests bt
	WHERE REGEXP_CONTAINS(bt.tags, 'owner_\\w+')
),
last_owner_t AS (
	SELECT
		target,
		LAST_VALUE(MIN(owner)) OVER(
			PARTITION BY target
			ORDER BY MIN(start_time) ASC
			ROWS BETWEEN UNBOUNDED PRECEDING AND UNBOUNDED FOLLOWING
		) AS last_owner
	FROM extracted_owner
	GROUP BY target
),
sum_times AS (
	SELECT
		SUM(execution_wall_time_ms) AS total_time,
		target
	FROM buildkite_analytics.bazel_tests bt
	JOIN buildkite_analytics.builds b
	ON bt.build_id = b.id
	WHERE b.started_at >= @start_time AND b.started_at < @end_time
	GROUP BY target
),
ranks AS (
	SELECT
		lo.*,
		ROUND(s.total_time/1000/60, 2) AS total_time,
		DENSE_RANK() OVER(PARTITION BY last_owner ORDER BY total_time DESC) AS rrank
	FROM sum_times s
	JOIN last_owner_t lo
	ON s.target = lo.target
),
cached_or_not AS (
	SELECT
	  CASE WHEN cache = 'UNCACHED' THEN 0 ELSE 1 END AS cached,
	  target
	FROM buildkite_analytics.bazel_tests bt
	JOIN buildkite_analytics.builds b
	ON bt.build_id = b.id
	WHERE
	  cache <> ''
	  AND TIMESTAMP_TRUNC(b.started_at, DAY) BETWEEN
	  	@start_time AND @end_time
  ),
cache_ratios AS (
	SELECT
		COUNTIF(cached = 1)/COUNT(*) AS cache_ratio,
		COUNT(*) AS runs,
		target
	FROM cached_or_not
	GROUP BY target
)
SELECT r.*, cr.cache_ratio, cr.runs
FROM ranks r
JOIN cache_ratios cr
ON r.target = cr.target
WHERE rrank < @n
ORDER BY last_owner, total_time DESC
`
