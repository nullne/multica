-- Generation tag used to bind a release/rollback to the same reservation
-- "epoch". A human reset bumps the generation so a stale rollback from a
-- pre-reset reservation cannot decrement the post-reset chain.
ALTER TABLE issue
    ADD COLUMN agent_mention_chain_generation BIGINT NOT NULL DEFAULT 0;
