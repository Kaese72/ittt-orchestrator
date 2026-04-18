UPDATE conditions SET timezone = 'UTC' WHERE timezone IS NULL;
ALTER TABLE conditions MODIFY COLUMN timezone VARCHAR(64) NOT NULL DEFAULT 'UTC';
