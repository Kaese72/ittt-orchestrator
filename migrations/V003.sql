CREATE TABLE IF NOT EXISTS rule_actions (
    id         INT AUTO_INCREMENT PRIMARY KEY,
    rule_id    INT          NOT NULL,
    type       VARCHAR(255) NOT NULL,
    target_id  INT          NOT NULL,
    capability VARCHAR(255) NOT NULL,
    args       JSON,
    FOREIGN KEY (rule_id) REFERENCES rules(id) ON DELETE CASCADE
);
