package mesa

import (
	"fmt"
	"strings"
)

// Markup compiles the producer-side markup grammar into a cell of spans:
//
//	<SEV [ATTR ...]>text</SEV>   SEV a severity name, ATTR in {b, i, u}
//
// Tags are flat (non-nesting); unmarked text compiles to a neutral span; a
// literal '<' is written "\<" and a literal '\' is written "\\". See RFC
// 0003 §9. Markup is an author-time convenience — malformed input panics,
// because producers MUST resolve it before serialization and the wire only
// ever carries compiled spans. Use [ParseMarkup] to handle errors instead.
func Markup(s string) Cell {
	spans, err := parseMarkup(s)
	if err != nil {
		panic("mesa.Markup: " + err.Error())
	}
	return Cell{Spans: spans}
}

// ParseMarkup is the non-panicking form of [Markup].
func ParseMarkup(s string) (Cell, error) {
	spans, err := parseMarkup(s)
	if err != nil {
		return Cell{}, err
	}
	return Cell{Spans: spans}, nil
}

func parseMarkup(s string) ([]Span, error) {
	var spans []Span
	var buf strings.Builder
	var cur Span
	open := false

	flush := func() {
		if buf.Len() == 0 {
			return
		}
		sp := cur
		sp.Text = buf.String()
		spans = append(spans, sp)
		buf.Reset()
	}

	i := 0
	for i < len(s) {
		switch s[i] {
		case '\\':
			if i+1 < len(s) && (s[i+1] == '<' || s[i+1] == '\\') {
				buf.WriteByte(s[i+1])
				i += 2
				continue
			}
			buf.WriteByte('\\')
			i++
		case '<':
			rel := strings.IndexByte(s[i:], '>')
			if rel < 0 {
				return nil, fmt.Errorf("unterminated tag at offset %d", i)
			}
			tag := s[i+1 : i+rel]
			i += rel + 1
			if strings.HasPrefix(tag, "/") {
				if !open {
					return nil, fmt.Errorf("close tag %q without an open tag", "<"+tag+">")
				}
				flush()
				cur = Span{}
				open = false
				continue
			}
			if open {
				return nil, fmt.Errorf("nested tag %q not allowed", "<"+tag+">")
			}
			flush()
			fields := strings.Fields(tag)
			if len(fields) == 0 {
				return nil, fmt.Errorf("empty tag <>")
			}
			sev, ok := ParseSeverity(fields[0])
			if !ok {
				return nil, fmt.Errorf("unknown severity %q", fields[0])
			}
			cur = Span{Sev: sev}
			for _, attr := range fields[1:] {
				switch attr {
				case "b":
					cur.Bold = true
				case "i":
					cur.Italic = true
				case "u":
					cur.Underline = true
				default:
					return nil, fmt.Errorf("unknown attribute %q", attr)
				}
			}
			open = true
		default:
			buf.WriteByte(s[i])
			i++
		}
	}
	if open {
		return nil, fmt.Errorf("unclosed tag at end of input")
	}
	flush()
	return spans, nil
}
