UPDATE user_subscriptions AS us
SET current_plan_name = sp.name
FROM subscription_plans AS sp
WHERE us.current_plan_id = sp.id
  AND us.current_plan_name IS DISTINCT FROM sp.name;
