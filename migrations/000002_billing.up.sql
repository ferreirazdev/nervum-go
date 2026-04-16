-- billing_plans: catalog of sellable plans (stripe_price_id must exist in Stripe Dashboard).
CREATE TABLE billing_plans (
    id                   UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    slug                 TEXT NOT NULL,
    name                 TEXT NOT NULL,
    description          TEXT NOT NULL DEFAULT '',
    stripe_price_id      TEXT NOT NULL,
    currency             TEXT NOT NULL DEFAULT 'usd',
    price_interval       TEXT NOT NULL DEFAULT 'month',
    display_amount_cents INT,
    features             JSONB,
    active               BOOLEAN NOT NULL DEFAULT true,
    sort_order           INT NOT NULL DEFAULT 0,
    created_at           TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at           TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at           TIMESTAMPTZ,
    CONSTRAINT uq_billing_plans_slug UNIQUE (slug)
);
CREATE INDEX idx_billing_plans_active_sort ON billing_plans (active, sort_order) WHERE deleted_at IS NULL;

-- organizations: Stripe subscription state (one subscription per org).
ALTER TABLE organizations
    ADD COLUMN stripe_customer_id TEXT NOT NULL DEFAULT '',
    ADD COLUMN stripe_subscription_id TEXT NOT NULL DEFAULT '',
    ADD COLUMN billing_plan_id UUID REFERENCES billing_plans (id),
    ADD COLUMN subscription_status TEXT NOT NULL DEFAULT 'none',
    ADD COLUMN trial_ends_at TIMESTAMPTZ,
    ADD COLUMN current_period_end TIMESTAMPTZ;

CREATE INDEX idx_organizations_stripe_subscription_id ON organizations (stripe_subscription_id)
    WHERE stripe_subscription_id <> '';

-- Seed example plan: replace stripe_price_id with a real Price id from Stripe before going live.
INSERT INTO billing_plans (slug, name, description, stripe_price_id, currency, price_interval, display_amount_cents, features, active, sort_order)
VALUES (
    'starter',
    'Starter',
    'For small teams getting started with system visibility.',
    'price_REPLACE_WITH_STRIPE_PRICE_ID',
    'usd',
    'month',
    4900,
    '["Up to 3 environments", "GitHub & GCP integrations", "Email support"]'::jsonb,
    true,
    0
);
