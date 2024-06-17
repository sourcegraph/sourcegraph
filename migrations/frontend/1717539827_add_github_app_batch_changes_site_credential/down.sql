-- delete the constraints
ALTER TABLE IF EXISTS batch_changes_site_credentials DROP CONSTRAINT IF EXISTS check_github_app_id_and_external_service_type;

-- delete the `github_app_id` column
ALTER TABLE IF EXISTS batch_changes_site_credentials DROP COLUMN IF EXISTS github_app_id;
