package a

type recv struct{}

// --- Diagnostics expected: three declarations share (int, string, bool) ---
// The receiver on charlie is ignored, so the method joins the group.

func alpha(x int, s string, b bool)            {} // want "shared by 3 declarations"
func bravo(x int, s string, b bool)            {} // want "shared by 3 declarations"
func (r recv) charlie(x int, s string, b bool) {} // want "shared by 3 declarations"

// --- No diagnostics: only two declarations share (float64, float64, float64) ---

func delta(x, y, z float64) {}
func echo(x, y, z float64)  {}

// --- No diagnostics: three declarations but only two parameters each ---
// (below the default -min-params of 3; this is where idiomatic shapes
// like (w http.ResponseWriter, r *http.Request) fall out).

func foxtrot(a, b int) {}
func golf(a, b int)    {}
func hotel(a, b int)   {}

// --- No diagnostics: one of three is suppressed, dropping the group to two ---

func india(p, q, r string)  {}
func juliet(p, q, r string) {}
func kilo(p, q, r string)   {} //paramobj:allow
