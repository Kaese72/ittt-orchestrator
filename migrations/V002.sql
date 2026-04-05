CREATE TABLE IF NOT EXISTS conditions (
    id               INT AUTO_INCREMENT PRIMARY KEY,
    rule_id          INT          NOT NULL,
    type             VARCHAR(255) NOT NULL,
    time             VARCHAR(8)   NULL,
    device_id        INT          NULL,
    attribute        VARCHAR(255) NULL,
    boolean          TINYINT(1)   NULL,
    and_condition_id INT          NULL,
    or_condition_id  INT          NULL,
    FOREIGN KEY (rule_id) REFERENCES rules(id) ON DELETE CASCADE
);
