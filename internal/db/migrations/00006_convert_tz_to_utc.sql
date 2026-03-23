-- +goose Up
-- Convert all existing timestamps to strict UTC strings by using SQLite's strftime.
-- This normalizes any timezone offset (-05:00, etc.) present in old strings into universal UTC.
-- If no offset was used (implying standard UTC fallback), it enforces the identical universally appended Z string format.

UPDATE users SET created_at = strftime('%Y-%m-%dT%H:%M:%SZ', created_at) WHERE created_at IS NOT NULL;
UPDATE users SET disabled_at = strftime('%Y-%m-%dT%H:%M:%SZ', disabled_at) WHERE disabled_at IS NOT NULL;

UPDATE user_credentials SET created_at = strftime('%Y-%m-%dT%H:%M:%SZ', created_at) WHERE created_at IS NOT NULL;
UPDATE user_credentials SET last_used_at = strftime('%Y-%m-%dT%H:%M:%SZ', last_used_at) WHERE last_used_at IS NOT NULL;

UPDATE sessions SET created_at = strftime('%Y-%m-%dT%H:%M:%SZ', created_at) WHERE created_at IS NOT NULL;
UPDATE sessions SET expires_at = strftime('%Y-%m-%dT%H:%M:%SZ', expires_at) WHERE expires_at IS NOT NULL;

UPDATE foods SET created_at = strftime('%Y-%m-%dT%H:%M:%SZ', created_at) WHERE created_at IS NOT NULL;
UPDATE foods SET deleted_at = strftime('%Y-%m-%dT%H:%M:%SZ', deleted_at) WHERE deleted_at IS NOT NULL;

UPDATE food_log_entries SET logged_at = strftime('%Y-%m-%dT%H:%M:%SZ', logged_at) WHERE logged_at IS NOT NULL;
UPDATE food_log_entries SET created_at = strftime('%Y-%m-%dT%H:%M:%SZ', created_at) WHERE created_at IS NOT NULL;
UPDATE food_log_entries SET deleted_at = strftime('%Y-%m-%dT%H:%M:%SZ', deleted_at) WHERE deleted_at IS NOT NULL;

-- +goose Down
-- Reversing this would require artificially shifting absolute correct UTC back to potentially wrong unknown varying offsets.
-- Therefore, we leave it strictly as UTC.
