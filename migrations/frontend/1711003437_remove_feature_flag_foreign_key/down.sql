-- Undo the changes made in the up migration

-- Drop first to make it idempotent
ALTER TABLE feature_flag_overrides
DROP CONSTRAINT IF EXISTS feature_flag_overrides_flag_name_fkey;

ALTER TABLE feature_flag_overrides
ADD CONSTRAINT feature_flag_overrides_flag_name_fkey
FOREIGN KEY (flag_name)
REFERENCES feature_flags(flag_name)
ON UPDATE CASCADE
ON DELETE CASCADE;
