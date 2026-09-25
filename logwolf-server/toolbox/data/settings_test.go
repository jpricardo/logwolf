package data

import "testing"

func TestLowersRetention(t *testing.T) {
	cases := []struct {
		from, to int
		want     bool
	}{
		{90, 30, true},
		{365, 180, true},
		{0, 365, true}, // forever to anything finite
		{0, 30, true},
		{30, 90, false},
		{180, 365, false},
		{365, 0, false}, // anything to forever
		{90, 90, false},
		{0, 0, false},
	}

	for _, tc := range cases {
		if got := LowersRetention(tc.from, tc.to); got != tc.want {
			t.Errorf("LowersRetention(%d, %d) = %v, want %v", tc.from, tc.to, got, tc.want)
		}
	}
}
