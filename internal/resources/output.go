package resources

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
)

// listFlags are shared by every list command.
type listFlags struct {
	limit   int
	jsonOut bool
}

func addListFlags(cmd *cobra.Command, f *listFlags) {
	cmd.Flags().IntVar(&f.limit, "limit", 50, "Maximum rows to return (server caps at 1000)")
	cmd.Flags().BoolVar(&f.jsonOut, "json", false, "Emit the raw JSON response")
}

// column maps a table header to a (possibly nested, dot-separated) key in
// the response rows.
type column struct {
	header string
	key    string
}

// printList renders PostgREST list bytes as a table, or verbatim
// (pretty-printed) with --json.
func printList(raw []byte, cols []column, jsonOut bool) error {
	if jsonOut {
		return printJSON(raw)
	}
	var rows []map[string]any
	if err := json.Unmarshal(raw, &rows); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	if len(rows) == 0 {
		fmt.Fprintln(os.Stderr, "No results.")
		return nil
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	headers := make([]string, len(cols))
	for i, c := range cols {
		headers[i] = strings.ToUpper(c.header)
	}
	fmt.Fprintln(w, strings.Join(headers, "\t"))
	for _, row := range rows {
		vals := make([]string, len(cols))
		for i, c := range cols {
			vals[i] = field(row, c.key)
		}
		fmt.Fprintln(w, strings.Join(vals, "\t"))
	}
	return w.Flush()
}

// printJSON pretty-prints raw response bytes.
func printJSON(raw []byte) error {
	var buf bytes.Buffer
	if err := json.Indent(&buf, raw, "", "  "); err != nil {
		_, werr := os.Stdout.Write(raw)
		return werr
	}
	buf.WriteByte('\n')
	_, err := buf.WriteTo(os.Stdout)
	return err
}

// field extracts a dot-path value from a row and formats it for a table
// cell. Timestamps collapse to their date part; nil becomes "".
func field(row map[string]any, path string) string {
	var v any = row
	for _, part := range strings.Split(path, ".") {
		m, ok := v.(map[string]any)
		if !ok {
			return ""
		}
		v = m[part]
	}
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		if ts, err := time.Parse(time.RFC3339, t); err == nil {
			return ts.Format("2006-01-02")
		}
		return t
	case bool:
		return strconv.FormatBool(t)
	case float64:
		return strconv.FormatFloat(t, 'f', -1, 64)
	default:
		b, err := json.Marshal(t)
		if err != nil {
			return fmt.Sprintf("%v", t)
		}
		return string(b)
	}
}
