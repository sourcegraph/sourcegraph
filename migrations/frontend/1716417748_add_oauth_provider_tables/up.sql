CREATE TABLE IF NOT EXISTS oauth_provider_client_applications (
    id                   SERIAL PRIMARY KEY,
    name                 TEXT NOT NULL,
    description          TEXT NOT NULL,
    client_id            TEXT NOT NULL,
    encryption_key_id    TEXT,
    client_secret        bytea NOT NULL,
    redirect_url         TEXT NOT NULL,
    creator_id           INTEGER REFERENCES users (id) ON DELETE SET NULL DEFERRABLE,
    created_at           TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at           TIMESTAMP NOT NULL DEFAULT NOW()
);
