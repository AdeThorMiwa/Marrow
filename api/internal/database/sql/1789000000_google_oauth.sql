-- +migrate Up
-- OAuth-only users have no password; only email/password accounts have a
-- hash. NULL is impossible to verify against (Login bails), so this is safe.
ALTER TABLE users ALTER COLUMN password_hash DROP NOT NULL;

-- +migrate Up
-- Federated logins map a provider-specific stable subject (Google's "sub"
-- claim) to a Marrow user. The (provider, subject) pair uniquely identifies
-- the external identity, and the user_id link is what connects it to the
-- Marrow account (created via email/password, or provisioned on first Google
-- sign-in).
CREATE TABLE IF NOT EXISTS oauth_identities (
    user_id  TEXT NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    provider TEXT NOT NULL,
    subject  TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (provider, subject)
);

-- +migrate Up
CREATE INDEX IF NOT EXISTS idx_oauth_identities_user_id ON oauth_identities (user_id);

-- +migrate Down
DROP TABLE IF EXISTS oauth_identities;

-- +migrate Down
ALTER TABLE users ALTER COLUMN password_hash SET NOT NULL;
