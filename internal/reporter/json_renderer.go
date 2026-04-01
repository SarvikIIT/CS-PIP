package reporter

import (
	"encoding/json"
	"io"
)

// RenderJSON encodes the report as indented JSON and writes it to w.
func RenderJSON(r Report, w io.Writer) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(r)
}
