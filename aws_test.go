package main

import "testing"

func TestPartition(t *testing.T) {
	for _, tt := range []struct {
		a []string
		b int
		r [][]string
	}{
		{
			[]string{}, 0, [][]string{},
		},
		{
			[]string{}, 1, [][]string{},
		},
		{
			[]string{"0"}, 0, [][]string{},
		},
		{
			[]string{"0", "1"}, 1, [][]string{[]string{"0"}, []string{"1"}},
		},
		{
			[]string{"0", "1", "2"}, 1, [][]string{[]string{"0"}, []string{"1"}, []string{"2"}},
		},
		{
			[]string{"0", "1", "2"}, 2, [][]string{[]string{"0", "1"}, []string{"2"}},
		},
	} {
		r := partition(tt.a, tt.b)

		if len(r) != len(tt.r) {
			t.Errorf("partition(%v, %d) => %v, want %v. Incorrect number of subslices", tt.a, tt.b, r, tt.r)
		}

		for i := 0; i < len(r); i++ {
			s := r[i]
			if len(s) != len(tt.r[i]) {
				t.Errorf("partition(%v, %d) => %v, want %v. Subslice %d incorrect length", tt.a, tt.b, r, tt.r, i)
			}

			for j := 0; j < len(s); j++ {
				if s[j] != tt.r[i][j] {
					t.Errorf("partition(%v, %d) => %v, want %v. Value at (%d,%d) incorrect", tt.a, tt.b, r, tt.r, i, j)
				}
			}
		}
	}
}
