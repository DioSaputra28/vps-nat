CREATE TABLE support_ticket_messages (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    ticket_id uuid NOT NULL REFERENCES support_tickets(id) ON DELETE RESTRICT,
    sender_type text NOT NULL CHECK (sender_type IN ('system', 'user', 'admin')),
    sender_id uuid,
    message text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    CHECK (
        (sender_type = 'system' AND sender_id IS NULL)
        OR
        (sender_type IN ('user', 'admin') AND sender_id IS NOT NULL)
    )
);
