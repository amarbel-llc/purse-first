package mesa

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
)

// The wire types mirror the JSON schema of RFC 0003. They are kept separate
// from the domain types so field names and omitempty rules match the spec
// exactly.

type wireColumn struct {
	Name   string `json:"name"`
	Role   string `json:"role"`
	Align  string `json:"align,omitempty"`
	Shrink int    `json:"shrink,omitempty"`
	Min    int    `json:"min,omitempty"`
}

type wireLegend struct {
	Sev   string `json:"sev"`
	Glyph string `json:"glyph"`
	Label string `json:"label"`
}

type wireHeader struct {
	V       int               `json:"v,omitempty"`
	Columns []wireColumn      `json:"columns"`
	Legend  []wireLegend      `json:"legend,omitempty"`
	Empty   string            `json:"empty,omitempty"`
	Palette map[string]string `json:"palette,omitempty"`
}

type wireSpan struct {
	Text string `json:"text"`
	Sev  string `json:"sev,omitempty"`
	B    bool   `json:"b,omitempty"`
	I    bool   `json:"i,omitempty"`
	U    bool   `json:"u,omitempty"`
}

type wireCell struct {
	Spans []wireSpan `json:"spans"`
}

type wireRow struct {
	Cells []json.RawMessage `json:"cells"`
}

func roleName(r Role) string {
	if r == Flex {
		return "flex"
	}
	return "pin"
}

func alignName(a Align) string {
	if a == Right {
		return "right"
	}
	return "" // left is the default; omit
}

// EncodeStream writes the table as an NDJSON stream: a header record then
// one record per row (RFC 0003 §1). HTML escaping is disabled so '<', '>',
// and '&' survive verbatim in cell text.
func EncodeStream(w io.Writer, t *Table) error {
	if err := t.validate(); err != nil {
		return err
	}
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)

	if err := enc.Encode(t.header()); err != nil {
		return err
	}
	for _, row := range t.Rows {
		cells := make([]json.RawMessage, len(row.Cells))
		for i, c := range row.Cells {
			raw, err := cellToWire(c)
			if err != nil {
				return err
			}
			cells[i] = raw
		}
		if err := enc.Encode(wireRow{Cells: cells}); err != nil {
			return err
		}
	}
	return nil
}

func (t *Table) header() wireHeader {
	h := wireHeader{Empty: t.EmptyText}
	if t.Version > 1 {
		h.V = t.Version
	}
	h.Columns = make([]wireColumn, len(t.Columns))
	for i, c := range t.Columns {
		h.Columns[i] = wireColumn{
			Name:   c.Name,
			Role:   roleName(c.Role),
			Align:  alignName(c.Align),
			Shrink: c.ShrinkPriority,
			Min:    c.MinWidth,
		}
	}
	if len(t.Legends) > 0 {
		h.Legend = make([]wireLegend, len(t.Legends))
		for i, e := range t.Legends {
			h.Legend[i] = wireLegend{Sev: e.Sev.String(), Glyph: e.Glyph, Label: e.Label}
		}
	}
	if len(t.PaletteOverride) > 0 {
		h.Palette = make(map[string]string, len(t.PaletteOverride))
		for sev, color := range t.PaletteOverride {
			h.Palette[sev.String()] = color
		}
	}
	return h
}

// marshalNoEscape marshals v to compact JSON without escaping '<', '>', or
// '&', matching the stream encoder so cell text survives verbatim.
func marshalNoEscape(v any) (json.RawMessage, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return json.RawMessage(bytes.TrimRight(buf.Bytes(), "\n")), nil
}

func cellToWire(c Cell) (json.RawMessage, error) {
	if c.isPlain() {
		return marshalNoEscape(c.plain())
	}
	wc := wireCell{Spans: make([]wireSpan, len(c.Spans))}
	for i, sp := range c.Spans {
		ws := wireSpan{Text: sp.Text, B: sp.Bold, I: sp.Italic, U: sp.Underline}
		if sp.Sev != Neutral {
			ws.Sev = sp.Sev.String()
		}
		wc.Spans[i] = ws
	}
	return marshalNoEscape(wc)
}

// DecodeStream reads an NDJSON stream into a [Table], enforcing the protocol
// errors of RFC 0003 §8: the first record must carry a non-empty columns
// list, versions above 1 are rejected, roles must be pin/flex, and every
// row's cell count must equal the column count.
func DecodeStream(r io.Reader) (*Table, error) {
	dec := json.NewDecoder(r)

	var h wireHeader
	if err := dec.Decode(&h); err != nil {
		if err == io.EOF {
			return nil, fmt.Errorf("mesa: empty stream (no header record)")
		}
		return nil, fmt.Errorf("mesa: header record: %w", err)
	}
	t, err := headerToTable(h)
	if err != nil {
		return nil, err
	}

	for {
		var row wireRow
		err := dec.Decode(&row)
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("mesa: row record: %w", err)
		}
		if len(row.Cells) != len(t.Columns) {
			return nil, fmt.Errorf("mesa: row has %d cells, want %d", len(row.Cells), len(t.Columns))
		}
		cells := make([]Cell, len(row.Cells))
		for i, raw := range row.Cells {
			c, err := wireToCell(raw)
			if err != nil {
				return nil, err
			}
			cells[i] = c
		}
		t.Rows = append(t.Rows, Row{Cells: cells})
	}
	return t, nil
}

func headerToTable(h wireHeader) (*Table, error) {
	if h.V != 0 && h.V != 1 {
		return nil, fmt.Errorf("mesa: unsupported protocol version %d", h.V)
	}
	if len(h.Columns) == 0 {
		return nil, fmt.Errorf("mesa: header record has no columns")
	}
	t := New()
	t.EmptyText = h.Empty
	for _, wc := range h.Columns {
		if wc.Name == "" {
			return nil, fmt.Errorf("mesa: column has empty name")
		}
		var role Role
		switch wc.Role {
		case "pin":
			role = Pin
		case "flex":
			role = Flex
		default:
			return nil, fmt.Errorf("mesa: column %q has invalid role %q", wc.Name, wc.Role)
		}
		align := Left
		if wc.Align == "right" {
			align = Right
		}
		t.Columns = append(t.Columns, Column{
			Name:           wc.Name,
			Role:           role,
			Align:          align,
			ShrinkPriority: wc.Shrink,
			MinWidth:       wc.Min,
		})
	}
	for _, wl := range h.Legend {
		sev, _ := ParseSeverity(wl.Sev) // unknown degrades to Neutral
		t.Legends = append(t.Legends, LegendEntry{Sev: sev, Glyph: wl.Glyph, Label: wl.Label})
	}
	if len(h.Palette) > 0 {
		t.PaletteOverride = make(map[Severity]string, len(h.Palette))
		for name, color := range h.Palette {
			if sev, ok := ParseSeverity(name); ok {
				t.PaletteOverride[sev] = color
			}
		}
	}
	return t, nil
}

func wireToCell(raw json.RawMessage) (Cell, error) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) > 0 && trimmed[0] == '"' {
		var s string
		if err := json.Unmarshal(raw, &s); err != nil {
			return Cell{}, fmt.Errorf("mesa: string cell: %w", err)
		}
		return Text(s), nil
	}
	var wc wireCell
	if err := json.Unmarshal(raw, &wc); err != nil {
		return Cell{}, fmt.Errorf("mesa: object cell: %w", err)
	}
	spans := make([]Span, len(wc.Spans))
	for i, ws := range wc.Spans {
		sev := Neutral
		if ws.Sev != "" {
			if s, ok := ParseSeverity(ws.Sev); ok {
				sev = s
			}
		}
		spans[i] = Span{Text: ws.Text, Sev: sev, Bold: ws.B, Italic: ws.I, Underline: ws.U}
	}
	return Cell{Spans: spans}, nil
}
