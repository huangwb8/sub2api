//go:build unit

package repository

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSplitSearchTerms(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		expect []string
	}{
		{
			name:   "empty",
			input:  "   ",
			expect: nil,
		},
		{
			name:   "split by spaces and punctuation",
			input:  "ab, dy；foo、bar",
			expect: []string{"ab", "dy", "foo", "bar"},
		},
		{
			name:   "deduplicate case insensitively",
			input:  "Prod prod PROD",
			expect: []string{"Prod"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.expect, splitSearchTerms(tt.input))
		})
	}
}
