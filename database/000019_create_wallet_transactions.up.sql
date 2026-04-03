CREATE TABLE wallet_transactions (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    wallet_id uuid NOT NULL REFERENCES wallets(id) ON DELETE RESTRICT,
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    direction text NOT NULL CHECK (direction IN ('credit', 'debit')),
    transaction_type text NOT NULL CHECK (transaction_type IN ('topup', 'payment', 'refund', 'admin_adjustment')),
    amount bigint NOT NULL CHECK (amount > 0),
    balance_before bigint NOT NULL CHECK (balance_before >= 0),
    balance_after bigint NOT NULL CHECK (balance_after >= 0),
    source_type text NOT NULL CHECK (source_type IN ('order', 'payment', 'wallet_topup', 'service', 'admin_adjustment', 'system', 'manual')),
    source_id uuid,
    note text,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX wallet_transactions_wallet_id_created_at_idx
    ON wallet_transactions (wallet_id, created_at DESC);
