package mesa

import "github.com/mattn/go-runewidth"

// spansWidth is the wcwidth-aware display width of spans' sanitized text.
func spansWidth(spans []Span) int {
	total := 0
	for _, sp := range spans {
		total += displayWidth(sanitize(sp.Text))
	}
	return total
}

// sameStyle reports whether two spans differ only in their text, so adjacent
// pieces of one wrapped line can be merged into a single styled run.
func sameStyle(a, b Span) bool {
	a.Text, b.Text = "", ""
	return a == b
}

// atom is a maximal run of spaces or of non-space text, possibly spanning
// several styled spans (a glyph span directly followed by a label span with
// no space between them is one word).
type atom struct {
	space  bool
	pieces []Span
	width  int
}

func splitAtoms(spans []Span) []atom {
	var atoms []atom
	for _, sp := range spans {
		var lastAtomPiece *Span
		for _, r := range sanitize(sp.Text) {
			isSpace := r == ' '
			if n := len(atoms); n == 0 || atoms[n-1].space != isSpace {
				atoms = append(atoms, atom{space: isSpace})
				lastAtomPiece = nil
			}
			a := &atoms[len(atoms)-1]
			if lastAtomPiece == nil {
				piece := sp
				piece.Text = ""
				a.pieces = append(a.pieces, piece)
				lastAtomPiece = &a.pieces[len(a.pieces)-1]
			}
			lastAtomPiece.Text += string(r)
			a.width += runewidth.RuneWidth(r)
		}
	}
	return atoms
}

// lineWrapper greedily fills lines of at most width display columns.
type lineWrapper struct {
	width   int
	lines   [][]Span
	cur     []Span
	curW    int
	hasWord bool
	pending *atom // a space held back until the next word is known to fit
}

func (w *lineWrapper) appendText(sp Span, text string, cols int) {
	if n := len(w.cur); n > 0 && sameStyle(w.cur[n-1], sp) {
		w.cur[n-1].Text += text
	} else {
		sp.Text = text
		w.cur = append(w.cur, sp)
	}
	w.curW += cols
}

func (w *lineWrapper) appendAtom(a atom) {
	for _, p := range a.pieces {
		w.appendText(p, p.Text, displayWidth(p.Text))
	}
}

func (w *lineWrapper) breakLine() {
	if w.hasWord {
		w.lines = append(w.lines, w.cur)
	}
	w.cur, w.curW, w.hasWord, w.pending = nil, 0, false, nil
}

func (w *lineWrapper) add(a atom) {
	if a.space {
		switch {
		case w.curW == 0 && len(w.lines) == 0:
			// Leading whitespace on the first line is the producer's
			// indentation; keep it.
			w.appendAtom(a)
		case w.curW == 0:
			// Never start a continuation line with the space that was
			// left at the break.
		default:
			w.pending = &a
		}
		return
	}

	need := a.width
	if w.pending != nil {
		need += w.pending.width
	}
	if w.curW+need <= w.width {
		if w.pending != nil {
			w.appendAtom(*w.pending)
			w.pending = nil
		}
		w.appendAtom(a)
		w.hasWord = true
		return
	}
	w.breakLine()
	if a.width <= w.width {
		w.appendAtom(a)
		w.hasWord = true
		return
	}
	// A single word wider than the width: break it at character boundaries.
	for _, p := range a.pieces {
		for _, r := range p.Text {
			rw := runewidth.RuneWidth(r)
			if w.curW+rw > w.width && w.curW > 0 {
				w.breakLine()
			}
			w.appendText(p, string(r), rw)
			w.hasWord = true
		}
	}
}

// wrapSpans breaks spans into lines of at most width display columns. It
// breaks at spaces (the space at a break is dropped) and keeps every span's
// styling; a word wider than width alone is split at character boundaries.
// A width of zero or less means unknown and a line that already fits is
// returned untouched, so callers render exactly what they did before wrapping
// existed.
func wrapSpans(spans []Span, width int) [][]Span {
	if width <= 0 || spansWidth(spans) <= width {
		return [][]Span{spans}
	}
	w := lineWrapper{width: width}
	for _, a := range splitAtoms(spans) {
		w.add(a)
	}
	w.breakLine()
	if len(w.lines) == 0 {
		// Only whitespace wider than the width: still one (blank) line.
		return [][]Span{nil}
	}
	return w.lines
}
