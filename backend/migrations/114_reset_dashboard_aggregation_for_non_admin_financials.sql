-- Dashboard financial aggregates now exclude admin users.
-- Reset the aggregation cursor so the scheduler recomputes retained buckets with the new cost semantics.
UPDATE usage_dashboard_aggregation_watermark
SET last_aggregated_at = '1970-01-01 00:00:00+00',
    updated_at = NOW()
WHERE id = 1;
