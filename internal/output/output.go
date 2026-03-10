package output

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"text/tabwriter"
)

var jsonMode bool

// SetJSONMode enables or disables JSON output mode.
func SetJSONMode(j bool) {
	jsonMode = j
}

// IsJSONMode reports whether JSON output mode is active.
func IsJSONMode() bool {
	return jsonMode
}

// Print routes to JSON output or calls humanFn for human-readable output.
func Print(v interface{}, humanFn func()) {
	if jsonMode {
		data, err := json.MarshalIndent(v, "", "  ")
		if err != nil {
			fmt.Fprintf(os.Stderr, "error marshaling JSON: %v\n", err)
			return
		}
		fmt.Println(string(data))
	} else {
		humanFn()
	}
}

// Table is a simple tabwriter-backed table.
type Table struct {
	w       *tabwriter.Writer
	headers []string
}

// NewTable creates a new Table writing to w with the given headers.
func NewTable(w io.Writer, headers []string) *Table {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	t := &Table{w: tw, headers: headers}
	// Print header row
	for i, h := range headers {
		if i > 0 {
			fmt.Fprint(tw, "\t")
		}
		fmt.Fprint(tw, h)
	}
	fmt.Fprintln(tw)
	// Print separator
	for i, h := range headers {
		if i > 0 {
			fmt.Fprint(tw, "\t")
		}
		for range h {
			fmt.Fprint(tw, "-")
		}
	}
	fmt.Fprintln(tw)
	return t
}

// Row writes a row to the table.
func (t *Table) Row(cols ...string) {
	for i, c := range cols {
		if i > 0 {
			fmt.Fprint(t.w, "\t")
		}
		fmt.Fprint(t.w, c)
	}
	fmt.Fprintln(t.w)
}

// Flush flushes the tabwriter.
func (t *Table) Flush() {
	t.w.Flush()
}

// IsTerminal reports whether stdout is a terminal (respects NO_COLOR).
func IsTerminal() bool {
	if os.Getenv("NO_COLOR") != "" {
		return false
	}
	fi, err := os.Stdout.Stat()
	if err != nil {
		return false
	}
	return (fi.Mode() & os.ModeCharDevice) != 0
}
