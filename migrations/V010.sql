ALTER TABLE rules
    ADD COLUMN backoff_duration_seconds BIGINT NULL,
    ADD COLUMN backoff_until            DATETIME(6) NULL;
