CREATE TABLE payments (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    order_id uuid REFERENCES orders(id) ON DELETE RESTRICT,
    wallet_topup_id uuid REFERENCES wallet_topups(id) ON DELETE RESTRICT,
    purpose text NOT NULL CHECK (purpose IN ('order', 'wallet_topup')),
    method text NOT NULL CHECK (method IN ('wallet', 'qris', 'manual')),
    provider text,
    provider_reference text,
    provider_payload jsonb NOT NULL DEFAULT '{}'::jsonb,
    amount bigint NOT NULL CHECK (amount > 0),
    status text NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'paid', 'failed', 'expired', 'canceled')),
    paid_at timestamptz,
    expired_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CHECK (((order_id IS NOT NULL)::int + (wallet_topup_id IS NOT NULL)::int) = 1),
    CHECK (
        (purpose = 'order' AND order_id IS NOT NULL AND wallet_topup_id IS NULL)
        OR
        (purpose = 'wallet_topup' AND wallet_topup_id IS NOT NULL AND order_id IS NULL)
    )
);

CREATE UNIQUE INDEX payments_one_paid_order_idx
    ON payments (order_id)
    WHERE status = 'paid' AND order_id IS NOT NULL;

CREATE UNIQUE INDEX payments_one_paid_wallet_topup_idx
    ON payments (wallet_topup_id)
    WHERE status = 'paid' AND wallet_topup_id IS NOT NULL;

CREATE INDEX payments_status_created_at_idx
    ON payments (status, created_at DESC);

CREATE INDEX payments_provider_reference_idx
    ON payments (provider_reference);
