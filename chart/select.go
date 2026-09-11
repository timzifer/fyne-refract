package chart

import (
	"github.com/timzifer/figure"
	"github.com/timzifer/figure/data"
	"github.com/timzifer/figure/geom"
	"github.com/timzifer/figure/interact"
	"github.com/timzifer/figure/ir"

	fynefigure "github.com/timzifer/fyne_figure"
)

// A selection is the rows a reader picked. figure holds none — it says what the
// pointer is on and where a row landed, and what that means is the program's
// (its ADR 0045) — so this is the program's half, and it is the same half the
// orbit widget keeps, spelled once in [fynefigure.Selection].
//
// What a widget adds is the three things a caller would otherwise write per
// chart: the click, the ring, and the one line each way that links two charts.

// Selection is the rows a reader has picked out.
func (c *Chart) Selection() fynefigure.Selection {
	c.lock.Lock()
	defer c.lock.Unlock()
	return c.sel.Clone()
}

// SetSelection puts a selection into the chart and redraws it.
//
// It deliberately fires no [Chart.OnSelect]: a selection that was put there is
// not a selection the reader made, and a pair of charts that told each other
// about their own would never stop. It is [Chart.SetView]'s rule for the same
// reason, and it is what makes the link one line each way.
//
// The rows are resolved against this chart's own layers when they are drawn, so
// a selection made in another chart over another table marks the right rows
// here as long as both name a key column — see
// [github.com/timzifer/figure/geom.KeyBy]. A row this chart cannot place is
// kept and not drawn, so panning it back into view brings its ring back.
func (c *Chart) SetSelection(sel fynefigure.Selection) {
	c.lock.Lock()
	defer c.lock.Unlock()
	c.setSelection(sel)
}

// setSelection installs a selection and redraws if anything changed. The lock
// is held by the caller, and no surface hold is: it draws.
func (c *Chart) setSelection(sel fynefigure.Selection) bool {
	if !c.applySelection(sel) {
		return false
	}
	c.draw()
	return true
}

// applySelection installs a selection and settles what figure has installed,
// without drawing, reporting whether anything changed.
//
// It is separate from [Chart.setSelection] because a click arrives *inside* the
// surface hold the release already took — the same constraint the legend toggle
// documents — so the frame a click needs is drawn by the release rather than
// here. Drawing from here would re-enter the hold.
//
// The lock is held by the caller.
func (c *Chart) applySelection(sel fynefigure.Selection) bool {
	if c.sel.Equal(sel) {
		return false
	}
	c.sel = sel.Clone()
	c.syncOverlay()
	return true
}

// OnSelect registers what to call when the reader picks rows out or clears
// them. Passing nil removes it.
//
// It is what links two charts, and the link is one line each way:
//
//	flat.OnSelect(func(s fynefigure.Selection) { scene.SetSelection(s) })
//	scene.OnSelect(func(s fynefigure.Selection) { flat.SetSelection(s) })
//
// That does not loop: [Chart.SetSelection] is not a reader picking anything and
// reports nothing back.
//
// The handler runs on the goroutine the click arrived on, with the chart it
// belongs to held — so it must not call back into *that* chart. Another chart
// is fine, which is the case this exists for.
func (c *Chart) OnSelect(fn func(fynefigure.Selection)) {
	c.lock.Lock()
	defer c.lock.Unlock()
	c.onSelect = fn
}

// SetSelect turns picking on or off for a chart already on screen, and clears
// what was picked when it turns it off. It is [Select] after construction.
func (c *Chart) SetSelect(on bool) {
	c.lock.Lock()
	defer c.lock.Unlock()
	if c.cfg.selects == on {
		return
	}
	was := c.tracksRows()
	c.cfg.selects = on
	if !on {
		c.setSelection(nil)
		return
	}
	if c.live != nil && !was {
		c.live.TrackRows(true)
		c.draw()
	}
}

// SetMultiSelect turns adding-to-the-selection on or off. It is [MultiSelect]
// after construction, and it is what a caller wanting shift-to-add calls from
// its own key handler.
func (c *Chart) SetMultiSelect(on bool) {
	c.lock.Lock()
	defer c.lock.Unlock()
	c.cfg.multiSelect = on
}

// clicked is what a click on a mark does to the selection. The lock is held by
// the caller, because the release that delivered the event holds it.
func (c *Chart) clicked(ev figure.Event) {
	if !c.cfg.selects || ev.Hit.Kind.Guides() {
		return
	}
	sel := c.sel
	if !ev.Found || ev.Hit.Row < 0 {
		// A click on nothing clears the selection, which is the gesture every
		// reader already knows and the only way to unpick the last row without
		// finding it again.
		sel = nil
	} else {
		ref := fynefigure.Ref{
			Key:   ev.Key,
			View:  ev.Hit.Panel,
			Layer: ev.Hit.Layer,
			Row:   ev.Hit.Row,
		}
		if c.cfg.multiSelect {
			sel = sel.Toggle(ref)
		} else if sel.Contains(ref) {
			sel = nil
		} else {
			sel = fynefigure.Selection{ref}
		}
	}
	if c.applySelection(sel) {
		// The release draws this frame; see [Chart.up]. Saying so rather than
		// drawing here is what keeps the click out of the surface hold it is
		// already inside.
		c.selDirty = true
		c.selected()
	}
}

// selected tells the handler what is picked now. The lock is held by the caller.
func (c *Chart) selected() {
	if c.onSelect != nil {
		c.onSelect(c.sel.Clone())
	}
}

// marks is the ring round every picked row, resolved against the frame that is
// being drawn.
//
// It resolves in DrawOverlay rather than when the selection changes, and that
// is the whole reason it is a type of its own. An overlay is drawn after every
// layer, so by the time this runs the hit index holds *this* frame's marks —
// which means a ring follows a pan, a zoom and a stream without anybody
// recomputing it, and a ring is never a frame behind the row it is round.
type marks struct {
	idx *interact.Index
	sel fynefigure.Selection

	// layers is the chart's layers, for reading a row's key back out. It is the
	// plot's slice rather than the panel's: a faceted chart's panels hold
	// Subset copies over one cut of the table, and a hit's row has already been
	// resolved back to the table the caller handed in.
	layers []geom.Geom

	// ring is what actually draws. Keeping one rather than making it per frame
	// is what stops a selection costing an allocation on every pointer move.
	ring figure.Highlight

	// at and rows are scratch, kept for the same reason.
	at   []ir.Point
	rows []interact.RowRef
}

// DrawOverlay implements [figure.Overlay].
func (m *marks) DrawOverlay(b ir.Backend, f figure.OverlayFrame) {
	if len(m.sel) == 0 || m.idx == nil {
		return
	}
	m.at = m.at[:0]
	for _, ref := range m.sel {
		m.at = m.locate(m.at, ref)
	}
	if len(m.at) == 0 {
		return
	}
	m.ring.At, m.ring.Panel = m.at, -1
	m.ring.DrawOverlay(b, f)
}

// locate appends wherever a ref landed in the frame just drawn, which may be
// nowhere — a row scrolled off the chart is not somewhere the reader can be
// pointed at — and may be more than one place on a faceted chart, where one row
// is drawn in one panel but a selection from another chart is not confined to
// it.
func (m *marks) locate(dst []ir.Point, ref fynefigure.Ref) []ir.Point {
	// The cheap path: a ref this chart made itself names a layer and a row of
	// this chart's own table, so the index can be asked straight out.
	if ref.Layer >= 0 && ref.Row >= 0 && m.keyAt(ref.Layer, ref.Row) == ref.Key {
		for p := range m.panels() {
			if at, ok := m.idx.Locate(p, ref.Layer, ref.Row); ok {
				dst = append(dst, at)
			}
		}
		return dst
	}
	if ref.Key == "" {
		return dst
	}
	// The other chart's ref: the key is all that crosses, so every row this
	// frame drew is asked what it is called. It is a pass over the marks on
	// screen rather than over the table — the index holds what was drawn — and
	// it happens only for a selection this chart did not make.
	for p := range m.panels() {
		for layer := range m.layers {
			m.rows = m.idx.RowsOf(p, layer, m.rows[:0])
			for _, r := range m.rows {
				if m.keyAt(layer, r.Row) == ref.Key {
					dst = append(dst, r.At)
				}
			}
		}
	}
	return dst
}

func (m *marks) panels() int { return len(m.idx.Panels()) }

// keyAt reads what a layer calls one of its rows, or "" for a layer that names
// no key column. It is [figure.Live]'s own keyOf, which is unexported — three
// calls, and the reason each of them is the reason figure gives.
func (m *marks) keyAt(layer, row int) string {
	if layer < 0 || row < 0 || layer >= len(m.layers) {
		return ""
	}
	g := m.layers[layer]
	col := geom.KeyOf(g)
	if col == "" {
		return ""
	}
	src, ok := geom.SourceOf(g)
	if !ok {
		return ""
	}
	key, _ := data.Label(src, col, row)
	return key
}
