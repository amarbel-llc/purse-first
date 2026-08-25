package mesa

import "fmt"

// Role is a column's width behavior.
type Role uint8

const (
	Pin  Role = iota // sized to content
	Flex             // absorbs and shrinks to fit the terminal
)

// Align is a column's horizontal alignment.
type Align uint8

const (
	Left Align = iota
	Right
)

// Column declares one column of the table.
type Column struct {
	Name           string
	Role           Role
	Align          Align
	ShrinkPriority int // among Flex columns: lower shrinks first
	MinWidth       int // Flex floor before ellipsizing (0 = renderer default)
}

// LegendEntry is one status-key row rendered in the footer.
type LegendEntry struct {
	Sev   Severity
	Glyph string
	Label string
}

// Entry constructs a [LegendEntry].
func Entry(sev Severity, glyph, label string) LegendEntry {
	return LegendEntry{Sev: sev, Glyph: glyph, Label: label}
}

// Table is a listing: columns, rows, an optional legend, empty-state text,
// and an optional per-severity palette override. Build it with the fluent
// methods and hand it to [Table.Render] or [EncodeStream].
type Table struct {
	Version         int
	Columns         []Column
	Legends         []LegendEntry
	EmptyText       string
	PaletteOverride map[Severity]string
	Rows            []Row
}

// New returns an empty v1 table.
func New() *Table {
	return &Table{Version: 1}
}

// ColOpt configures a [Column].
type ColOpt func(*Column)

// WithAlign sets a column's alignment.
func WithAlign(a Align) ColOpt { return func(c *Column) { c.Align = a } }

// Shrink sets a Flex column's shrink priority (lower shrinks first).
func Shrink(priority int) ColOpt { return func(c *Column) { c.ShrinkPriority = priority } }

// MinWidth sets a Flex column's minimum width before ellipsizing.
func MinWidth(n int) ColOpt { return func(c *Column) { c.MinWidth = n } }

// Col appends a column.
func (t *Table) Col(name string, role Role, opts ...ColOpt) *Table {
	c := Column{Name: name, Role: role}
	for _, o := range opts {
		o(&c)
	}
	t.Columns = append(t.Columns, c)
	return t
}

// Legend appends legend entries rendered as a footer.
func (t *Table) Legend(entries ...LegendEntry) *Table {
	t.Legends = append(t.Legends, entries...)
	return t
}

// Empty sets the text rendered when the table has no rows.
func (t *Table) Empty(msg string) *Table {
	t.EmptyText = msg
	return t
}

// Palette overrides the color of one or more severities for this table.
// Values are "#RRGGBB" hex or a base-10 ANSI 256 index.
func (t *Table) Palette(p map[Severity]string) *Table {
	t.PaletteOverride = p
	return t
}

// Row appends a row. Its cell count must equal the column count; the
// mismatch is reported by [Table.Render] and [EncodeStream].
func (t *Table) Row(cells ...Cell) *Table {
	t.Rows = append(t.Rows, Row{Cells: cells})
	return t
}

// validate checks structural invariants shared by rendering and encoding.
func (t *Table) validate() error {
	if len(t.Columns) == 0 {
		return fmt.Errorf("mesa: table has no columns")
	}
	for i, r := range t.Rows {
		if len(r.Cells) != len(t.Columns) {
			return fmt.Errorf("mesa: row %d has %d cells, want %d", i, len(r.Cells), len(t.Columns))
		}
	}
	return nil
}

// columnNames returns the sanitized header labels.
func (t *Table) columnNames() []string {
	names := make([]string, len(t.Columns))
	for i, c := range t.Columns {
		names[i] = sanitize(c.Name)
	}
	return names
}
