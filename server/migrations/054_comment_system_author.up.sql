DO $$
DECLARE
    _constraint text;
BEGIN
    SELECT con.conname INTO _constraint
    FROM pg_constraint con
    JOIN pg_class cls ON con.conrelid = cls.oid
    WHERE cls.relname = 'comment'
      AND con.contype = 'c'
      AND pg_get_constraintdef(con.oid) LIKE '%author_type%';

    IF _constraint IS NOT NULL THEN
        EXECUTE 'ALTER TABLE comment DROP CONSTRAINT ' || quote_ident(_constraint);
    END IF;
END $$;

ALTER TABLE comment ADD CONSTRAINT comment_author_type_check
    CHECK (author_type IN ('member', 'agent', 'system'));
