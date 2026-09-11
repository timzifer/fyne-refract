package fynefigure

// A selection is what a reader picked out, and it is the one piece of chart
// state that belongs to neither chart.
//
// figure supplies the two ends of the wire and no link: a hit says which row
// the pointer is on, [github.com/timzifer/figure/interact.Index.Locate] says
// where a row landed, and what a selection *means* is the program's — see
// figure's ADR 0045 and ADR 0062, which decide it for two charts and for two
// cameras respectively. This package is a host, so this is the program's half,
// written once rather than once per widget.
//
// It lives in the root package because both widgets speak it. A selection made
// in a projected scene and shown in a flat chart beside it is the case the
// whole arrangement exists for, and a type in either package would make one of
// the two import the other.

// Ref is one row a reader picked: which row, and what it is called.
//
// Two of them name the same row when both carry a non-empty Key and the keys
// are equal, and otherwise when their layer and row agree. The key is what
// crosses a table — [github.com/timzifer/figure/geom.KeyBy] names the column
// that identifies a row, so two charts drawn from two tables can agree about
// one measurement — and the layer and row are the fallback for a layer that
// names no key, where the only honest claim is about one table.
//
// View is deliberately not part of identity. A figure with four cameras is one
// chart looked at four ways, so a row picked in the plan is the same row in the
// three-quarter view; View records where the reader was pointing when they
// picked it, which is worth keeping and is not what makes it that row.
type Ref struct {
	// Key is the row's identity, or "" for a layer that names no key column.
	Key string
	// View is the panel or view the reader picked in, or -1 when it came from
	// somewhere that was not a pointer.
	View int
	// Layer is which layer of it, and Row the source row in the table that
	// layer was given. Row is -1 when the hit reported none, which is what an
	// index that was not tracking rows reports.
	Layer, Row int
}

// Named reports whether the ref carries an identity that crosses tables.
func (r Ref) Named() bool { return r.Key != "" }

// Same reports whether two refs name one row.
func (r Ref) Same(o Ref) bool {
	if r.Named() && o.Named() {
		return r.Key == o.Key
	}
	if r.Named() != o.Named() {
		return false
	}
	return r.Layer == o.Layer && r.Row == o.Row && r.Row >= 0
}

// Selection is the set of rows a reader has picked, in the order they picked
// them.
//
// It is a slice rather than a map because it is small, ordered and compared far
// more often than it is searched: a widget asks "is this the same selection as
// last frame" on every pointer move and "is this row in it" once per row it
// draws. The order is worth keeping — the first row picked is the one a caller
// showing a single reading shows.
//
// The zero Selection is empty and usable.
type Selection []Ref

// Contains reports whether the selection names a row.
func (s Selection) Contains(r Ref) bool { return s.IndexOf(r) >= 0 }

// IndexOf reports where a row is in the selection, or -1.
func (s Selection) IndexOf(r Ref) int {
	for i, have := range s {
		if have.Same(r) {
			return i
		}
	}
	return -1
}

// Add returns the selection with a row added, or unchanged when it is already
// there. It does not modify the receiver, so a handler may keep the value it
// was handed.
func (s Selection) Add(r Ref) Selection {
	if s.Contains(r) {
		return s
	}
	out := make(Selection, len(s), len(s)+1)
	copy(out, s)
	return append(out, r)
}

// Remove returns the selection without a row, or unchanged when it was not
// there.
func (s Selection) Remove(r Ref) Selection {
	i := s.IndexOf(r)
	if i < 0 {
		return s
	}
	out := make(Selection, 0, len(s)-1)
	out = append(out, s[:i]...)
	return append(out, s[i+1:]...)
}

// Toggle adds a row that is not selected and removes one that is, which is what
// a click with a modifier held usually means.
func (s Selection) Toggle(r Ref) Selection {
	if s.Contains(r) {
		return s.Remove(r)
	}
	return s.Add(r)
}

// Equal reports whether two selections name the same rows in the same order.
//
// It is what a widget asks before telling anybody that the selection changed,
// so that a click landing on the row that was already picked is not an event —
// and, with the no-echo rule the two widgets follow, what keeps two linked
// charts from telling each other about a selection for ever.
func (s Selection) Equal(o Selection) bool {
	if len(s) != len(o) {
		return false
	}
	for i := range s {
		if s[i] != o[i] {
			return false
		}
	}
	return true
}

// Clone returns a copy, for a caller keeping a selection past the call it
// arrived in.
func (s Selection) Clone() Selection {
	if len(s) == 0 {
		return nil
	}
	out := make(Selection, len(s))
	copy(out, s)
	return out
}
