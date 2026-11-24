-- The devices and groups each rule refers to (restmodels.Rule.RelatedIDs),
-- stored on the rule as sorted JSON arrays of IDs and rewritten whenever the
-- rule is saved, so rules can be listed by the device or group they involve
-- (the related-devices / related-groups filters, via JSON_OVERLAPS).
ALTER TABLE rules
    ADD COLUMN related_devices JSON NOT NULL DEFAULT '[]',
    ADD COLUMN related_groups  JSON NOT NULL DEFAULT '[]';

-- Backfill existing rules, with the same rules as RelatedIDs: devices from
-- conditions and device-capability actions, groups from group-capability
-- actions, positive IDs only, sorted and de-duplicated (UNION removes the
-- duplicates between conditions and actions, DISTINCT those within each).
UPDATE rules r SET related_devices = COALESCE((
    SELECT JSON_ARRAYAGG(DISTINCT d.id ORDER BY d.id)
    FROM (
        SELECT rule_id, device_id AS id FROM conditions WHERE device_id > 0
        UNION
        SELECT rule_id, target_id AS id FROM rule_actions WHERE type = 'device-capability' AND target_id > 0
    ) d
    WHERE d.rule_id = r.id
), '[]');
UPDATE rules r SET related_groups = COALESCE((
    SELECT JSON_ARRAYAGG(DISTINCT a.target_id ORDER BY a.target_id)
    FROM rule_actions a
    WHERE a.rule_id = r.id AND a.type = 'group-capability' AND a.target_id > 0
), '[]');
