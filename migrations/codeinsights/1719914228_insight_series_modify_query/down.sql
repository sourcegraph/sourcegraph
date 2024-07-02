DO
$$
    BEGIN
        -- Check if column 'query_old' exists
        IF EXISTS (SELECT 1
                   FROM information_schema.columns
                   WHERE table_name = 'insight_series' AND column_name = 'query_old') THEN
            -- Update the 'query' column with values from 'query_old'
            UPDATE insight_series SET query = query_old;
            ALTER TABLE insight_series DROP COLUMN query_old;
        END IF;
    END
$$;
