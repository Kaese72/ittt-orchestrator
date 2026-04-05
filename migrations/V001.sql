CREATE TABLE IF NOT EXISTS rules (
    id                INT AUTO_INCREMENT PRIMARY KEY,
    name              VARCHAR(255) NOT NULL,
    enabled           TINYINT(1)   NOT NULL DEFAULT 1,
    root_condition_id INT          NULL
);
