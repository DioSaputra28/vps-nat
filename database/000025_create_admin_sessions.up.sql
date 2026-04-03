CREATE TABLE admin_sessions (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    admin_user_id uuid NOT NULL REFERENCES admin_users(id) ON DELETE RESTRICT,
    token_hash text NOT NULL UNIQUE,
    user_agent text,
    ip_address inet,
    expires_at timestamptz NOT NULL,
    last_used_at timestamptz,
    revoked_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CHECK (expires_at > created_at)
);

CREATE INDEX admin_sessions_admin_user_id_idx
    ON admin_sessions (admin_user_id);

CREATE INDEX admin_sessions_expires_at_idx
    ON admin_sessions (expires_at);

CREATE INDEX admin_sessions_revoked_at_idx
    ON admin_sessions (revoked_at);
