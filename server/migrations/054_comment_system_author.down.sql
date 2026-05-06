UPDATE comment SET author_type = 'agent' WHERE author_type = 'system';

ALTER TABLE comment DROP CONSTRAINT IF EXISTS comment_author_type_check;
ALTER TABLE comment ADD CONSTRAINT comment_author_type_check
    CHECK (author_type IN ('member', 'agent'));
