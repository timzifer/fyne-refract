package fynefigure

import "testing"

// A key crosses tables and a row number does not, so two refs that both name a
// key are compared by it and nothing else. This is what lets a selection made
// in a projected scene be shown in a flat chart drawn from another table.
func TestAKeyedRefIsTheSameRowWhereverItWasFound(t *testing.T) {
	a := Ref{Key: "s7", View: 0, Layer: 1, Row: 42}
	b := Ref{Key: "s7", View: 3, Layer: 0, Row: 9}
	if !a.Same(b) {
		t.Error("two refs with one key are not the same row")
	}
	if a.Same(Ref{Key: "s8", Layer: 1, Row: 42}) {
		t.Error("two refs with different keys are the same row")
	}
}

// A layer that names no key can still be pointed at, and then the only honest
// claim is about one table: the same layer and the same row.
func TestAnUnkeyedRefFallsBackToItsLayerAndRow(t *testing.T) {
	a := Ref{View: 0, Layer: 1, Row: 42}
	if !a.Same(Ref{View: 2, Layer: 1, Row: 42}) {
		t.Error("one row of one layer is not itself when seen from another view")
	}
	if a.Same(Ref{View: 0, Layer: 2, Row: 42}) {
		t.Error("two layers' rows are the same row")
	}
	if a.Same(Ref{Key: "s7", Layer: 1, Row: 42}) {
		t.Error("a named row and an unnamed one at the same index are the same row")
	}
}

// A hit that reported no row names nothing, and must not collide with every
// other such hit.
func TestARefWithNoRowNamesNothing(t *testing.T) {
	a := Ref{View: 0, Layer: 0, Row: -1}
	if a.Same(Ref{View: 0, Layer: 0, Row: -1}) {
		t.Error("two hits that found no row are the same row")
	}
}

func TestASelectionAddsRemovesAndToggles(t *testing.T) {
	var s Selection
	one := Ref{Key: "a"}
	two := Ref{Key: "b"}

	s = s.Add(one).Add(two).Add(one)
	if len(s) != 2 {
		t.Fatalf("adding one row twice gave %d rows", len(s))
	}
	if s[0] != one || s[1] != two {
		t.Error("a selection did not keep the order the rows were picked in")
	}
	if s = s.Toggle(one); s.Contains(one) {
		t.Error("toggling a picked row did not unpick it")
	}
	if s = s.Toggle(one); !s.Contains(one) {
		t.Error("toggling an unpicked row did not pick it")
	}
	if got := s.Remove(Ref{Key: "z"}); len(got) != len(s) {
		t.Error("removing a row that is not there changed the selection")
	}
}

// The value a handler was given stays what it was, so a widget can hand one out
// and a caller can keep it.
func TestAddingDoesNotEditTheSelectionItWasGiven(t *testing.T) {
	s := Selection{{Key: "a"}}
	held := s
	if got := s.Add(Ref{Key: "b"}); len(got) != 2 {
		t.Fatalf("add gave %d rows", len(got))
	}
	if len(held) != 1 || held[0].Key != "a" {
		t.Error("add edited the selection it was given")
	}
}

// Equal is what stops a link looping and what stops a click on an
// already-picked row being an event.
func TestEqualComparesRowsAndOrder(t *testing.T) {
	a := Selection{{Key: "a"}, {Key: "b"}}
	if !a.Equal(Selection{{Key: "a"}, {Key: "b"}}) {
		t.Error("two selections of the same rows are not equal")
	}
	if a.Equal(Selection{{Key: "b"}, {Key: "a"}}) {
		t.Error("order is not part of a selection")
	}
	if a.Equal(nil) || Selection(nil).Equal(a) {
		t.Error("an empty selection equals a full one")
	}
	if !Selection(nil).Equal(Selection{}) {
		t.Error("two empty selections are not equal")
	}
}
