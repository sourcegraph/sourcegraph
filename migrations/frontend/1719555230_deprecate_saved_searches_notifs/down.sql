-- no default value
ALTER TABLE saved_searches ALTER COLUMN notify_owner DROP DEFAULT;
ALTER TABLE saved_searches ALTER COLUMN notify_slack DROP DEFAULT;
