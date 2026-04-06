ALTER TABLE conditions
    DROP COLUMN time,
    ADD COLUMN from_time VARCHAR(8) NULL,
    ADD COLUMN to_time   VARCHAR(8) NULL;
