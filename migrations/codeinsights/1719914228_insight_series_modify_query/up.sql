ALTER TABLE insight_series ADD COLUMN IF NOT EXISTS query_old TEXT;
UPDATE insight_series SET query_old = query;

-- prefix query with patternType:standard if there is no patterntype: in the query
UPDATE insight_series
SET query = 'patterntype:standard ' || query
WHERE query NOT ILIKE '%patterntype:%'
  -- exclude empty queries, which are created by language stats insights
  AND query != '';

COMMENT ON COLUMN insight_series.query_old IS 'Backup for migration. Remove with release 5.6 or later.';
