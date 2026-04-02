package cmd

import (
    "fmt"
    "sort"

    "github.com/rtalexk/demux/internal/format"
    "github.com/rtalexk/demux/internal/query"
    "github.com/spf13/cobra"
)

var sessionNameOnly bool

var queryCmd = &cobra.Command{
    Use:   "query <term>",
    Short: "Fuzzy search sessions, windows, and processes",
    Args:  cobra.ExactArgs(1),
    RunE:  runQuery,
}

func init() {
    queryCmd.Flags().BoolVar(&sessionNameOnly, "session-name-only", false,
        "Output unique session names only (for piping to fzf)")
    rootCmd.AddCommand(queryCmd)
}

type queryRow struct {
    typ, session, window, score, matchedIn string
}

func (r queryRow) Fields() []string {
    return []string{r.typ, r.session, r.window, r.score, r.matchedIn}
}

func runQuery(cmd *cobra.Command, args []string) error {
    pq := query.Parse(args[0])
    result, err := query.Run(pq)
    if err != nil {
        return err
    }

    type flatRow struct {
        score     int
        typ       string
        session   string
        window    string
        matchedIn string
    }
    var flat []flatRow

    for _, sm := range result.Sessions {
        if len(sm.MatchPos) > 0 {
            flat = append(flat, flatRow{
                score:     sm.Score,
                typ:       "session",
                session:   sm.Name,
                window:    "-",
                matchedIn: "s:" + sm.Name,
            })
        }
        for _, wm := range sm.Windows {
            flat = append(flat, flatRow{
                score:     wm.Score,
                typ:       "window",
                session:   sm.Name,
                window:    fmt.Sprintf("%d", wm.Index),
                matchedIn: "w:" + wm.Name,
            })
        }
        for _, pm := range sm.Procs {
            flat = append(flat, flatRow{
                score:     pm.Score,
                typ:       "process",
                session:   sm.Name,
                window:    "-",
                matchedIn: "p:" + pm.Name,
            })
        }
    }

    sort.Slice(flat, func(i, j int) bool {
        return flat[i].score > flat[j].score
    })

    if sessionNameOnly {
        seen := make(map[string]bool)
        for _, r := range flat {
            if !seen[r.session] {
                seen[r.session] = true
                fmt.Println(r.session)
            }
        }
        return nil
    }

    headers := []string{"TYPE", "SESSION", "WINDOW", "SCORE", "MATCHED IN"}
    rows := make([]format.Row, len(flat))
    for i, r := range flat {
        rows[i] = queryRow{
            typ:       r.typ,
            session:   r.session,
            window:    r.window,
            score:     fmt.Sprintf("%d", r.score),
            matchedIn: r.matchedIn,
        }
    }

    isTTYVal := isTTY()
    fmt.Print(format.Render(resolveFormat(cmd), headers, rows, isTTYVal))
    return nil
}
