//go:build unit

package service

import "testing"

// TestBuildUsageBillingCommand_SubscriptionAppliesRateMultiplier locks in the
// fix that subscription-mode billing honours the configured multiplier:
// SubscriptionCostUSD must track ActualCost, not raw TotalCost.
func TestBuildUsageBillingCommand_SubscriptionAppliesRateMultiplier(t *testing.T) {
	t.Parallel()

	groupID := int64(7)
	subID := int64(42)

	tests := []struct {
		name           string
		totalCost      float64
		actualCost     float64
		isSubscription bool
		wantSub        float64
		wantBalance    float64
	}{
		{
			name:           "subscription with 2x multiplier consumes 2x quota",
			totalCost:      1.0,
			actualCost:     2.0,
			isSubscription: true,
			wantSub:        2.0,
			wantBalance:    0,
		},
		{
			name:           "subscription with 0.5x multiplier consumes 0.5x quota",
			totalCost:      1.0,
			actualCost:     0.5,
			isSubscription: true,
			wantSub:        0.5,
			wantBalance:    0,
		},
		{
			name:           "free subscription consumes no quota",
			totalCost:      1.0,
			actualCost:     0,
			isSubscription: true,
			wantSub:        0,
			wantBalance:    0,
		},
		{
			name:           "balance billing keeps using actual cost",
			totalCost:      1.0,
			actualCost:     2.0,
			isSubscription: false,
			wantSub:        0,
			wantBalance:    2.0,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			p := &postUsageBillingParams{
				Cost:               &CostBreakdown{TotalCost: tt.totalCost, ActualCost: tt.actualCost},
				User:               &User{ID: 1},
				APIKey:             &APIKey{ID: 2, GroupID: &groupID},
				Account:            &Account{ID: 3},
				Subscription:       &UserSubscription{ID: subID},
				IsSubscriptionBill: tt.isSubscription,
			}

			cmd := buildUsageBillingCommand("req-1", nil, p)
			if cmd == nil {
				t.Fatal("buildUsageBillingCommand returned nil")
			}
			if cmd.SubscriptionCostUSD != tt.wantSub {
				t.Fatalf("SubscriptionCostUSD = %v, want %v", cmd.SubscriptionCostUSD, tt.wantSub)
			}
			if cmd.BalanceCostUSD != tt.wantBalance {
				t.Fatalf("BalanceCostUSD = %v, want %v", cmd.BalanceCostUSD, tt.wantBalance)
			}
		})
	}
}

func TestBuildUsageBillingCommand_CarriesBalanceOverdraftGuard(t *testing.T) {
	t.Parallel()

	p := &postUsageBillingParams{
		Cost:                   &CostBreakdown{TotalCost: 1, ActualCost: 1},
		ChargeSnapshot:         &UsageChargeSnapshot{ChargedAmountCNY: 7.2},
		User:                   &User{ID: 1},
		APIKey:                 &APIKey{ID: 2},
		Account:                &Account{ID: 3},
		MaxBalanceOverdraftCNY: 0.8,
	}

	cmd := buildUsageBillingCommand("req-guard", nil, p)
	if cmd == nil {
		t.Fatal("buildUsageBillingCommand returned nil")
	}
	if cmd.BalanceCostCNY != 7.2 {
		t.Fatalf("BalanceCostCNY = %v, want 7.2", cmd.BalanceCostCNY)
	}
	if cmd.MaxBalanceOverdraftCNY != 0.8 {
		t.Fatalf("MaxBalanceOverdraftCNY = %v, want 0.8", cmd.MaxBalanceOverdraftCNY)
	}
}
