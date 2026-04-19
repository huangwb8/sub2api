-- 回填历史订阅 usage 的 estimated_cost_cny，避免盈利面板长期把订阅成本统计为 0。
-- 幂等执行：仅更新 billing_type=subscription 且 estimated_cost_cny 为空的历史行。

UPDATE usage_logs AS ul
SET estimated_cost_cny = ROUND(ul.actual_cost * (a.actual_cost_cny / a.actual_cost_usage_usd), 8)
FROM accounts AS a
WHERE ul.account_id = a.id
  AND ul.billing_type = 1
  AND ul.estimated_cost_cny IS NULL
  AND ul.actual_cost IS NOT NULL
  AND ul.actual_cost > 0
  AND a.actual_cost_cny IS NOT NULL
  AND a.actual_cost_cny > 0
  AND a.actual_cost_usage_usd IS NOT NULL
  AND a.actual_cost_usage_usd > 0;
