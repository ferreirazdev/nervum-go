DELETE FROM billing_plans WHERE slug = 'starter';

DROP INDEX IF EXISTS idx_organizations_stripe_subscription_id;

ALTER TABLE organizations
    DROP COLUMN IF EXISTS current_period_end,
    DROP COLUMN IF EXISTS trial_ends_at,
    DROP COLUMN IF EXISTS subscription_status,
    DROP COLUMN IF EXISTS billing_plan_id,
    DROP COLUMN IF EXISTS stripe_subscription_id,
    DROP COLUMN IF EXISTS stripe_customer_id;

DROP INDEX IF EXISTS idx_billing_plans_active_sort;
DROP TABLE IF EXISTS billing_plans;
