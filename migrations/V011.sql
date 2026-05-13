ALTER TABLE rules
    DROP COLUMN backoff_duration_seconds,
    DROP COLUMN backoff_until,
    ADD COLUMN cooldown_until DATETIME(6) NULL;

ALTER TABLE conditions
    ADD COLUMN cooldown_seconds BIGINT NULL;
