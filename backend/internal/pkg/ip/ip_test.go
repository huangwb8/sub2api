package ip

import "testing"

func TestNormalizeRiskEvidenceIP(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		raw  string
		want string
	}{
		{
			name: "public ipv4 stays unchanged",
			raw:  "203.0.113.10",
			want: "203.0.113.10",
		},
		{
			name: "public ipv4 strips port",
			raw:  "203.0.113.10:443",
			want: "203.0.113.10",
		},
		{
			name: "public ipv6 is normalized to slash 64",
			raw:  "2001:db8:abcd:12:3456:789a:bcde:f012",
			want: "2001:db8:abcd:12::/64",
		},
		{
			name: "private ipv4 is ignored",
			raw:  "10.0.0.8",
			want: "",
		},
		{
			name: "loopback is ignored",
			raw:  "127.0.0.1",
			want: "",
		},
		{
			name: "invalid ip is ignored",
			raw:  "not-an-ip",
			want: "",
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := NormalizeRiskEvidenceIP(tc.raw); got != tc.want {
				t.Fatalf("NormalizeRiskEvidenceIP(%q) = %q, want %q", tc.raw, got, tc.want)
			}
		})
	}
}
