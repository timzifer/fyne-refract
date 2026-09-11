package orbit

import (
	"fyne.io/fyne/v2"
	"github.com/timzifer/figure/data"
	"github.com/timzifer/figure/interact"
	"github.com/timzifer/figure/ir"
	"github.com/timzifer/figure/three"

	fynefigure "github.com/timzifer/fyne_figure"
)

// A selection in a projected scene is the one interaction the arrangement was
// built for and the one it could not have.
//
// figure's ADR 0062 gives a scene several views of one repository of data and
// closes by saying that marking a row in every one of them is the host's. It
// is — and until three gained an overlay there was nowhere to draw it. Here is
// the host's half: a click picks the row a hit reports, and the ring goes into
// every view at once, because a row picked in the plan is the same row in the
// three-quarter view.

// Selection is the rows a reader has picked out.
func (c *Chart) Selection() fynefigure.Selection {
	c.lock.Lock()
	defer c.lock.Unlock()
	return c.sel.Clone()
}

// SetSelection puts a selection into the scene and redraws it.
//
// It deliberately fires no [Chart.OnSelect], which is [Chart.SetCamera]'s rule
// for [Chart.SetCamera]'s reason: a selection that was put there is not a
// selection the reader made, and a pair of charts that told each other about
// their own would never stop.
//
// A row is resolved against this scene's own layers, so a selection made in a
// flat chart over another table marks the right rows here as long as both name
// a key column — see [github.com/timzifer/figure/geom.KeyBy]. A row this scene
// cannot place is kept and not drawn; turning the view until it is visible
// brings its ring back.
func (c *Chart) SetSelection(sel fynefigure.Selection) {
	c.lock.Lock()
	defer c.lock.Unlock()
	if !c.applySelection(sel) {
		return
	}
	c.draw()
}

// applySelection installs a selection and settles what figure has installed,
// without drawing, reporting whether anything changed. The lock is held by the
// caller.
func (c *Chart) applySelection(sel fynefigure.Selection) bool {
	if c.sel.Equal(sel) {
		return false
	}
	c.sel = sel.Clone()
	c.syncOverlay()
	return true
}

// syncOverlay gives figure the overlay the chart's state calls for, and nothing
// when that is nothing.
//
// The nothing matters here more than it does for a flat chart. A turn with an
// overlay installed repaints the whole canvas rather than the cells whose
// cameras moved — see [three.Live.Overlay] — so a scene carrying a permanently
// empty overlay would give up the partial repaint that makes a four-view figure
// affordable to drag, and get nothing for it. The lock is held by the caller.
func (c *Chart) syncOverlay() {
	if c.live == nil {
		return
	}
	if len(c.sel) == 0 {
		c.live.Overlay(nil)
		return
	}
	c.mk.idx, c.mk.sel = c.live.Index(), c.sel
	c.mk.views = c.live.ViewCount()
	c.live.Overlay(c.mk)
}

// OnSelect registers what to call when the reader picks rows out or clears
// them. Passing nil removes it.
//
// It is what links a scene to a chart beside it, and the link is one line each
// way:
//
//	scene.OnSelect(func(s fynefigure.Selection) { flat.SetSelection(s) })
//	flat.OnSelect(func(s fynefigure.Selection) { scene.SetSelection(s) })
//
// That does not loop: [Chart.SetSelection] is not a reader picking anything and
// reports nothing back.
//
// The handler runs on Fyne's goroutine with this chart held, so it must not
// call back into this chart; calling into another is what it is for.
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
		if c.applySelection(nil) {
			c.draw()
		}
		return
	}
	if c.live != nil && !was {
		c.live.TrackRows(true)
		c.draw()
	}
}

// SetMultiSelect turns adding-to-the-selection on or off. It is [MultiSelect]
// after construction.
func (c *Chart) SetMultiSelect(on bool) {
	c.lock.Lock()
	defer c.lock.Unlock()
	c.cfg.multiSelect = on
}

// tracksRows reports whether the chart should record which source row is behind
// each mark.
//
// [TrackRows] asks for it, and so does [Select]: in a projected scene the row
// is the *whole* answer a pointer has — there are no screen axes to invert a
// device point through — so a selection without it would pick nothing at all.
func (c *Chart) tracksRows() bool { return c.cfg.trackRows || c.cfg.selects }

// clicked is what a press and release that did not turn anything does to the
// selection. The lock is held by the caller.
func (c *Chart) clicked(pos fyne.Position) {
	if !c.cfg.selects || c.live == nil {
		return
	}
	sel := c.sel
	h, found := c.live.Index().At(ir.Point{X: pos.X, Y: pos.Y}, hoverSlop)
	if !found || h.Row < 0 {
		// A click on nothing clears the selection, which is the gesture every
		// reader already knows and the only way to unpick the last row without
		// finding it again.
		sel = nil
	} else {
		ref := fynefigure.Ref{
			Key:   c.keyOf(h.Panel, h.Layer, h.Row),
			View:  h.Panel,
			Layer: h.Layer,
			Row:   h.Row,
		}
		if c.cfg.multiSelect {
			sel = sel.Toggle(ref)
		} else if sel.Contains(ref) {
			sel = nil
		} else {
			sel = fynefigure.Selection{ref}
		}
	}
	if !c.applySelection(sel) {
		return
	}
	c.draw()
	if c.onSelect != nil {
		c.onSelect(c.sel.Clone())
	}
}

// keyOf reads what a layer of one view calls one of its rows, or "" for a layer
// that names no key column.
//
// It is [figure.Live]'s own keyOf, which is unexported, over a scene's layers
// instead of a chart's: a three layer says what it plots through
// [three.Describer], and the description carries both the key column and the
// source. The lock is held by the caller.
func (c *Chart) keyOf(view, layer, row int) string {
	g := c.layerAt(view, layer)
	if g == nil || row < 0 {
		return ""
	}
	d, ok := g.(three.Describer)
	if !ok {
		return ""
	}
	desc := d.Describe()
	if desc.Key == "" || desc.Source == nil {
		return ""
	}
	key, _ := data.Label(desc.Source, desc.Key, row)
	return key
}

// layerAt is one layer of the scene one view looks at, resolving the view's own
// scene against the plot's exactly as the drawing does.
func (c *Chart) layerAt(view, layer int) three.Layer {
	sc := c.plot.CurrentScene()
	if views := c.plot.Views(); view >= 0 && view < len(views) && views[view].Scene != nil {
		sc = views[view].Scene
	}
	if sc == nil || layer < 0 {
		return nil
	}
	ls := sc.Layers()
	if layer >= len(ls) {
		return nil
	}
	return ls[layer]
}

// marks is the ring round every picked row, in every view, resolved against the
// frame being drawn.
//
// Resolving here rather than when the selection changes is what makes the rings
// follow a turn: an overlay is drawn after every view, so the hit index it
// reads holds *this* frame's marks, and a camera the reader is dragging carries
// its rings with it without anybody recomputing them.
type marks struct {
	idx   *interact.Index
	sel   fynefigure.Selection
	views int

	// ring is what draws, kept rather than made per frame — a selection is
	// redrawn on every frame of a drag.
	ring three.Highlight

	at []ir.Point
}

// DrawOverlay implements [three.Overlay].
func (m *marks) DrawOverlay(b ir.Backend, f three.OverlayFrame) {
	if len(m.sel) == 0 || m.idx == nil {
		return
	}
	m.at = m.at[:0]
	for _, ref := range m.sel {
		if ref.Layer < 0 || ref.Row < 0 {
			// A row that came from a flat chart names a layer of *that* chart.
			// The key would have to be matched against this scene's rows to
			// place it, and a scene's marks are its lattice rather than its
			// table — so it is kept and not drawn.
			continue
		}
		// Every view, not the one the reader picked in. That is the whole
		// point: one scene looked at four ways, and one measurement marked in
		// all four.
		for view := range m.views {
			if at, ok := m.idx.Locate(view, ref.Layer, ref.Row); ok {
				m.at = append(m.at, at)
			}
		}
	}
	if len(m.at) == 0 {
		return
	}
	m.ring.At, m.ring.Data, m.ring.View = m.at, nil, -1
	m.ring.DrawOverlay(b, f)
}
