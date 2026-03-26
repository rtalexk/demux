package format

import (
    "encoding/json"
    "strings"
)

func JSON(headers []string, rows []Row) string {
    var sb strings.Builder
    for i, row := range rows {
        fields := row.Fields()
        m := make(map[string]string, len(headers))
        for j, h := range headers {
            if j < len(fields) {
                m[h] = fields[j]
            }
        }
        b, _ := json.Marshal(m)
        sb.Write(b)
        if i < len(rows)-1 {
            sb.WriteString("\n")
        }
    }
    return sb.String()
}
