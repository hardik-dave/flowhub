package main

import "testing"

func TestParseKeep(t *testing.T) {
	cases := []struct {
		name    string
		in      string
		want    []int64
		wantErr bool
	}{
		{name: "default", in: "1,11,12", want: []int64{1, 11, 12}},
		{name: "spaces and blanks", in: " 1 , , 11 ,12 ", want: []int64{1, 11, 12}},
		{name: "single", in: "7", want: []int64{7}},
		{name: "empty", in: "   ", wantErr: true},
		{name: "non-numeric", in: "1,abc", wantErr: true},
		{name: "zero", in: "0,1", wantErr: true},
		{name: "negative", in: "-3", wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseKeep(tc.in)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("parseKeep(%q) = %v, want error", tc.in, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseKeep(%q) error: %v", tc.in, err)
			}
			if len(got) != len(tc.want) {
				t.Fatalf("parseKeep(%q) = %v, want %v", tc.in, got, tc.want)
			}
			for _, id := range tc.want {
				if !got[id] {
					t.Fatalf("parseKeep(%q) missing id %d (got %v)", tc.in, id, got)
				}
			}
		})
	}
}
