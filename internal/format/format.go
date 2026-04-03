package format

import "strings"

// Row is implemented by any struct that can produce ordered field values.
type Row interface {
	Fields() []string
}

// Render dispatches to the correct formatter.
func Render(fmtName string, headers []string, rows []Row, isTTY bool) string {
	switch strings.ToLower(fmtName) {
	case "table":
		return Table(headers, rows, isTTY)
	case "json":
		return JSON(headers, rows)
	default:
		return Text(headers, rows)
	}
}
