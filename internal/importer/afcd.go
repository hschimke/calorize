package importer

import (
	"fmt"
	"strings"

	"github.com/xuri/excelize/v2"
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

// parseGroupInfo reads the first sheet of the Food Group Info xlsx and returns
// a map of food group ID → food group name.
// Scans for a header row containing "Food group ID" to handle leading informational rows.
func parseGroupInfo(f *excelize.File) (map[string]string, error) {
	sheetList := f.GetSheetList()
	if len(sheetList) == 0 {
		return make(map[string]string), nil
	}
	rows, err := f.GetRows(sheetList[0])
	if err != nil {
		return nil, fmt.Errorf("reading food group sheet: %w", err)
	}
	result := make(map[string]string)
	idCol, nameCol := -1, -1
	dataStart := -1
	for i, row := range rows {
		for j, cell := range row {
			switch strings.TrimSpace(cell) {
			case "Food group ID":
				idCol = j
			case "Food group name":
				nameCol = j
			}
		}
		if idCol >= 0 && nameCol >= 0 {
			dataStart = i + 1
			break
		}
	}
	if dataStart < 0 {
		return result, nil
	}
	for _, row := range rows[dataStart:] {
		id := cellStr(row, idCol)
		name := cellStr(row, nameCol)
		if id != "" && name != "" {
			result[id] = name
		}
	}
	return result, nil
}
