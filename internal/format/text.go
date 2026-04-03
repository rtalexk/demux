package format

import "strings"

func Text(headers []string, rows []Row) string {
	var sb strings.Builder
	for i, row := range rows {
		fields := row.Fields()
		for j, h := range headers {
			val := ""
			if j < len(fields) {
				val = fields[j]
			}
			sb.WriteString(h + ": " + val + "\n")
		}
		if i < len(rows)-1 {
			sb.WriteString("\n")
		}
	}
	return sb.String()
}
