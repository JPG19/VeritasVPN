CREATE EXTENSION IF NOT EXISTS "pgcrypto";

CREATE TABLE subscriptions (
    id                    UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    account_id            TEXT NOT NULL UNIQUE,
    tier                  TEXT NOT NULL DEFAULT 'free' CHECK (tier IN ('free', 'premium')),
    status                TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'canceled', 'past_due')),
    payment_method        TEXT NOT NULL DEFAULT 'stripe' CHECK (payment_method IN ('stripe', 'btcpay')),
    current_period_start  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    current_period_end    TIMESTAMPTZ NOT NULL,
    cancel_at_period_end  BOOLEAN NOT NULL DEFAULT FALSE,
    created_at            TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at            TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_subscriptions_account ON subscriptions(account_id);
CREATE INDEX idx_subscriptions_status ON subscriptions(status);

CREATE TABLE payment_records (
    id                     UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    subscription_id        UUID NOT NULL REFERENCES subscriptions(id) ON DELETE CASCADE,
    amount                 BIGINT NOT NULL,
    currency               TEXT NOT NULL DEFAULT 'usd',
    status                 TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'completed', 'failed', 'refunded')),
    provider_transaction_id TEXT NOT NULL,
    created_at             TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_payment_records_subscription ON payment_records(subscription_id);
CREATE INDEX idx_payment_records_provider_txn ON payment_records(provider_transaction_id);
