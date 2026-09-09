package chart

import (
	"github.com/timzifer/figure"
	"github.com/timzifer/figure/ir"
)

// The interaction surfaces figure offers: an overlay layer to paint
// over a finished chart, layers a reader can turn off, and a view that can be
// read off one chart and put into another.
//
// All three are figure's, and none of them is wired by figure — a legend
// that always toggled, a crosshair nobody asked for and two charts that always
// moved together would each be wrong somewhere. What a widget adds is the
// wiring, the surface hold every redraw needs, and the repaint afterwards.

// overlays is the caller's overlay with the rubber band of a drag over it.
//
// Both are optional and either may be nil, which is why this exists rather
// than [figure.Overlays]: the chart wants one installed thing whose parts it
// can change without reinstalling it, and a slice would have to be rebuilt
// every time a drag started.
type overlays struct {
	user  figure.Overlay
	brush *figure.Brush
}

// DrawOverlay implements [figure.Overlay]. The band goes last, so a selection
// being dragged out is drawn over a crosshair rather than under it.
func (o *overlays) DrawOverlay(b ir.Backend, f figure.OverlayFrame) {
	if o.user != nil {
		o.user.DrawOverlay(b, f)
	}
	if o.brush != nil {
		o.brush.DrawOverlay(b, f)
	}
}

// Overlay installs something to paint over the chart, replacing whatever was
// there, and redraws. Passing nil removes it.
//
// It is [figure.Live.Overlay] with the chart's own hold on the surface and a
// repaint afterwards. The overlay is a pointer to a struct whose fields the
// caller then moves — a crosshair's position, a tooltip's text — so installing
// it once and writing to it from a [figure.Hover] handler is the intended
// shape:
//
//	cross := &figure.Crosshair{}
//	c.Overlay(cross)
//	c.Plot().On(figure.Hover, func(ev figure.Event) {
//		cross.At, cross.Show = ev.Hit.At, ev.Found
//	})
//
// A hover redraws the chart while an overlay is installed and does not while
// one is not, so a chart that has no use for one should not install a
// do-nothing overlay to keep the code uniform.
func (c *Chart) Overlay(o figure.Overlay) {
	c.lock.Lock()
	defer c.lock.Unlock()
	c.overlay = o
	c.syncOverlay()
	c.draw()
}

// CurrentOverlay reports what the caller installed, or nil. It is not what
// figure was given: the chart composes that overlay with the band of a drag.
func (c *Chart) CurrentOverlay() figure.Overlay {
	c.lock.Lock()
	defer c.lock.Unlock()
	return c.overlay
}

// syncOverlay gives figure the overlay the chart's current state calls for,
// and nothing when that is nothing.
//
// The nothing matters. [figure.Input.Move] redraws the chart on every hover
// while an overlay is installed, because a handler that just moved a crosshair
// has no other way of being seen — so a chart carrying a permanently empty
// overlay would pay a frame per pointer move for a layer that draws nothing.
// The band is therefore installed when a drag starts and taken away when it
// ends. The lock is held by the caller.
func (c *Chart) syncOverlay() {
	if c.live == nil {
		return
	}
	band := c.brush
	if !c.banding {
		band = nil
	}
	if c.overlay == nil && band == nil {
		c.live.Overlay(nil)
		return
	}
	c.ov.user, c.ov.brush = c.overlay, band
	c.live.Overlay(c.ov)
}

// bands reports whether a drag paints a rubber band. A pan moves the chart and
// has nothing to outline, and a caller who passed [Brush] nil wants no band at
// all.
func (c *Chart) bands() bool {
	return c.cfg.drag != figure.DragPans && c.brush != nil
}

// band puts the brush where the drag currently is, and reports whether there
// is a band to draw. The lock is held by the caller.
func (c *Chart) band() bool {
	if !c.banding || c.brush == nil || c.in == nil {
		return false
	}
	r, ok := c.in.Dragged()
	if !ok {
		r = ir.Rect{}
	}
	c.brush.Rect = r
	return true
}

// endBand takes the rubber band away and settles what figure has installed.
//
// It is called before a release is reported, so that the frame the release
// draws is already free of the band — and again whenever a Live is born, since
// a chart rebuilt for a new typeface midway through a drag would otherwise
// start life with the rectangle of a gesture nobody is making any more. The
// lock is held by the caller.
func (c *Chart) endBand() {
	c.banding = false
	if c.brush != nil {
		c.brush.Rect = ir.Rect{}
	}
	c.syncOverlay()
}

// SetDragMode changes what a drag does after the chart was built. It is
// [DragMode] for a chart already on screen — a toolbar switching between
// panning and selecting is the case.
//
// A drag in progress is not converted: the mode is read when the next press
// starts.
//
// Switching *to* [figure.DragSelects] turns row tracking on and draws a frame,
// because a selection reads rows out of the hit index and an index that was not
// tracking them holds none. Switching away leaves it on: a chart that has been
// selected over once is a chart that will be again, and a caller who wants the
// index small again rebuilds. See [Chart.tracksRows].
func (c *Chart) SetDragMode(m figure.Drag) {
	c.lock.Lock()
	defer c.lock.Unlock()
	was := c.tracksRows()
	c.cfg.drag = m
	if c.in != nil {
		c.in.Drag(m)
	}
	if c.live != nil && !was && c.tracksRows() {
		c.live.TrackRows(true)
		c.draw()
	}
}

// HideLayer turns a layer off, or on again, and redraws.
//
// The name carries "layer" because [fyne.Widget] already has a Hide, which
// hides the whole widget — a chart with a Hide that hid one series would be a
// widget whose Hide meant something else than every other widget's. The three
// calls beside it are spelled to match.
//
// A hidden layer is not drawn and is not hit-tested, so a pointer where it was
// finds whatever is behind it. It still trains its scales and still has its
// legend row, dimmed — the axes deliberately do not move, because a toggle is
// a reading aid and an axis that rescaled on every click would make the two
// readings incomparable. A caller who does want the axes to follow what is
// left is saying something else, and says it with [figure.Plot.SetLayers] and
// [Chart.Rebuild].
func (c *Chart) HideLayer(layer int, hide bool) error {
	c.lock.Lock()
	defer c.lock.Unlock()
	return c.visibility(func() error { return c.live.Hide(layer, hide) })
}

// ToggleLayer hides a layer if it is shown and shows it if it is hidden. It is
// what [LegendToggle] wires a click on a legend row to.
func (c *Chart) ToggleLayer(layer int) error {
	c.lock.Lock()
	defer c.lock.Unlock()
	return c.visibility(func() error { return c.live.Toggle(layer) })
}

// ShowAllLayers brings every hidden layer back.
func (c *Chart) ShowAllLayers() error {
	c.lock.Lock()
	defer c.lock.Unlock()
	return c.visibility(func() error { return c.live.ShowAll() })
}

// LayerHidden reports whether a layer is currently turned off. It is false
// before the first layout, where there is no chart to hide anything in.
func (c *Chart) LayerHidden(layer int) bool {
	c.lock.Lock()
	defer c.lock.Unlock()
	if c.live == nil {
		return false
	}
	return c.live.IsHidden(layer)
}

// visibility runs a change that redraws, holding the surface and showing the
// frame. The lock is held by the caller.
func (c *Chart) visibility(fn func() error) error {
	if c.live == nil {
		return nil
	}
	if err := c.target.Render(fn); err != nil {
		c.renderr = err
		return err
	}
	c.present()
	return nil
}

// View is where the chart is currently looking: the domain of every axis of
// every panel.
//
// It is a value, so it stays true after the reader moves on, and it describes
// the chart it was taken from and nothing else — put one into a chart with a
// different number of panels and it is ignored. The zero View is what a chart
// that has not been laid out yet reports.
func (c *Chart) View() figure.View {
	c.lock.Lock()
	defer c.lock.Unlock()
	if c.live == nil {
		return figure.View{}
	}
	return c.live.View()
}

// SetView puts a view back and redraws. It is the other half of [Chart.View],
// and the two together are what links one chart to another.
//
// It deliberately fires no [Chart.OnViewChange]: a view that was put there is
// not a view the reader moved to, and a pair of charts that told each other
// about their own changes would never stop.
func (c *Chart) SetView(v figure.View) error {
	c.lock.Lock()
	defer c.lock.Unlock()
	if c.live == nil {
		return nil
	}
	if err := c.target.Render(func() error { return c.live.SetView(v) }); err != nil {
		c.renderr = err
		return err
	}
	c.present()
	return nil
}

// OnViewChange registers a handler for the reader moving the chart: a pan, a
// zoom, or the double click that puts it back. Passing nil removes it.
//
// It is what links two charts, and the link is one line each way:
//
//	left.OnViewChange(func(v figure.View) { right.SetView(v) })
//	right.OnViewChange(func(v figure.View) { left.SetView(v) })
//
// That does not loop: [Chart.SetView] is not a reader moving anything and
// reports nothing back.
//
// The handler runs on the goroutine the gesture arrived on, with the chart it
// belongs to held — so it must not call back into *that* chart. Another chart
// is fine, which is the case this exists for. A pan reports on every step of
// itself, at the rate the pacing lets frames through.
func (c *Chart) OnViewChange(fn func(figure.View)) {
	c.lock.Lock()
	defer c.lock.Unlock()
	c.onView = fn
}

// viewChanged tells the handler where the chart is now. The lock is held by
// the caller, or the call comes from inside a gesture that holds it.
func (c *Chart) viewChanged() {
	if c.onView == nil || c.live == nil {
		return
	}
	c.onView(c.live.View())
}
