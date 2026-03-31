package importer

import (
	"strings"
)

// parseUnitFromHeader extracts the unit from a column header like "Calcium (mg)" → "mg".
// Returns "" when no parenthetical unit is present.
func parseUnitFromHeader(header string) string {
	start := strings.LastIndex(header, "(")
	end := strings.LastIndex(header, ")")
	if start == -1 || end == -1 || end <= start+1 {
		return ""
	}
	return strings.TrimSpace(header[start+1 : end])
}

// cellStr safely reads and trims a string from a row slice by column index.
func cellStr(row []string, idx int) string {
	if idx < 0 || idx >= len(row) {
		return ""
	}
	return strings.TrimSpace(row[idx])
}
