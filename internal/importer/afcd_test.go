package importer

import (
	"testing"
)

func TestParseUnitFromHeader(t *testing.T) {
	cases := []struct{ header, want string }{
		{"Calcium (mg)", "mg"},
		{"Energy with dietary fibre, equated (kJ)", "kJ"},
		{"Protein (g)", "g"},
		{"Public Food Key", ""},
		{"", ""},
	}
	for _, c := range cases {
		got := parseUnitFromHeader(c.header)
		if got != c.want {
			t.Errorf("parseUnitFromHeader(%q) = %q, want %q", c.header, got, c.want)
		}
	}
}
