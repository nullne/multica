ALTER TABLE issue ADD COLUMN criteria_status TEXT
    CHECK (criteria_status IN ('pending', 'approved'));
