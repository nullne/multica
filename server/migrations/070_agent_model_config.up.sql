-- Per-provider model configuration for agents. Keyed by provider, e.g.
-- {"claude": {"model": "claude-fable-5", "thinking_level": "high"}}.
-- Empty object means "no overrides" — each provider CLI picks its own default.
ALTER TABLE agent ADD COLUMN model_config JSONB NOT NULL DEFAULT '{}';
