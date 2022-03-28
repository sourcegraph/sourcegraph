TRUNCATE TABLE user_pending_permissions;
TRUNCATE TABLE repo_pending_permissions;

ALTER TABLE IF EXISTS user_pending_permissions ALTER COLUMN id TYPE BIGINT;
ALTER TABLE IF EXISTS repo_pending_permissions ALTER COLUMN user_ids_ints TYPE BIGINT[];

ALTER SEQUENCE IF EXISTS user_external_accounts_id_seq RESTART;

