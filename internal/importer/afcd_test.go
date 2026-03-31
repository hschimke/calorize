package importer

import (
	"testing"

	"github.com/xuri/excelize/v2"
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

func TestParseGroupInfo(t *testing.T) {
	f := excelize.NewFile()
	defer f.Close()

	// Row 1: informational noise (like the real AFCD file)
	f.SetCellValue("Sheet1", "A1", "Some general note about this database")
	// Row 2: actual column headers
	f.SetCellValue("Sheet1", "A2", "Food group ID")
	f.SetCellValue("Sheet1", "B2", "Food group name")
	f.SetCellValue("Sheet1", "C2", "Inclusions")
	// Data rows
	f.SetCellValue("Sheet1", "A3", "7")
	f.SetCellValue("Sheet1", "B3", "Vegetable products and dishes")
	f.SetCellValue("Sheet1", "A4", "31")
	f.SetCellValue("Sheet1", "B4", "Cereal-based products and dishes")

	groups, err := parseGroupInfo(f)
	if err != nil {
		t.Fatalf("parseGroupInfo: %v", err)
	}
	if got := groups["7"]; got != "Vegetable products and dishes" {
		t.Errorf("groups[7] = %q, want %q", got, "Vegetable products and dishes")
	}
	if got := groups["31"]; got != "Cereal-based products and dishes" {
		t.Errorf("groups[31] = %q, want %q", got, "Cereal-based products and dishes")
	}
	if len(groups) != 2 {
		t.Errorf("len(groups) = %d, want 2", len(groups))
	}
}
