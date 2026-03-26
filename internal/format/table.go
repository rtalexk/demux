package format

import (
	"fmt"
	"strings"
)

const ansibold = "\033[1m"
const ansireset = "\033[0m"

func Table(headers []string, rows []Row, isTTY bool) string {
	widths := make([]int, len(headers))
	for i, h := range headers {
		widths[i] = len(h)
	}
	allFields := make([][]string, len(rows))
	for r, row := range rows {
		fields := row.Fields()
		allFields[r] = fields
		for i := 0; i < len(headers) && i < len(fields); i++ {
			if len(fields[i]) > widths[i] {
				widths[i] = len(fields[i])
			}
		}
	}

	var sb strings.Builder
	for i, h := range headers {
		if isTTY {
			sb.WriteString(ansibold)
		}
		sb.WriteString(fmt.Sprintf("%-*s", widths[i], h))
		if isTTY {
			sb.WriteString(ansireset)
		}
		if i < len(headers)-1 {
			sb.WriteString("  ")
		}
	}
	sb.WriteString("\n")

	for _, fields := range allFields {
		for i := 0; i < len(headers); i++ {
			val := ""
			if i < len(fields) {
				val = fields[i]
			}
			sb.WriteString(fmt.Sprintf("%-*s", widths[i], val))
			if i < len(headers)-1 {
				sb.WriteString("  ")
			}
		}
		sb.WriteString("\n")
	}
	return strings.TrimRight(sb.String(), "\n")
}
