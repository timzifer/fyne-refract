package chart

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/widget"
)

// The pointer. Every handler hands the position to figure.Input and then asks
// for a frame; Live paints nothing when the frame is identical to the last, so
// a hover that found the same mark twice costs nothing.
//
// Both Hoverable and Draggable are implemented because Fyne splits the pointer
// between them: while a button is held on a draggable object, MouseMoved stops
// firing and Dragged fires instead. figure.Input wants one stream of
// positions and works out for itself whether it is a hover or a pan.
//
// None of it is the chart's own. The handlers belong to pointer, a layer laid
// over the raster that is hidden unless the chart was made [Interactive]. Fyne
// decides where an event goes by the interfaces an object implements, not by
// whether it wants the event, so a chart that were Scrollable itself would take
// the wheel from the scroll container around it even while ignoring every
// notch. A hidden layer is not found at all, and the event goes to whatever is
// behind it.

// pointer is the layer that takes the pointer for a chart.
type pointer struct {
	widget.BaseWidget
	c *Chart
}

// The interfaces the layer answers. Tapping is deliberately not among them: a
// click already arrives through MouseUp, where figure's own click slop decides
// whether it was one, and fyne.Tappable would deliver a second copy of it a
// double-click delay later.
var (
	_ fyne.Widget         = (*pointer)(nil)
	_ fyne.Draggable      = (*pointer)(nil)
	_ fyne.Scrollable     = (*pointer)(nil)
	_ fyne.DoubleTappable = (*pointer)(nil)
	_ desktop.Hoverable   = (*pointer)(nil)
	_ desktop.Mouseable   = (*pointer)(nil)
	_ desktop.Cursorable  = (*pointer)(nil)
)

func newPointer(c *Chart, on bool) *pointer {
	p := &pointer{c: c}
	p.Hidden = !on
	p.ExtendBaseWidget(p)
	return p
}

// CreateRenderer is called by Fyne. It is not part of the API. The layer draws
// nothing; it is there to be found.
func (p *pointer) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(container.NewWithoutLayout())
}

// SetInteractive lets a reader at the chart, or takes the pointer back from
// them. It is [Interactive] for a chart already on screen.
//
// Taking it back ends whatever the reader was doing as a pointer leaving the
// chart would — a drag in progress is released, a tooltip hidden — and leaves
// the view where they put it. [Chart.Autoscale] is the way back from there.
func (c *Chart) SetInteractive(on bool) {
	c.lock.Lock()
	was := c.cfg.interactive
	c.cfg.interactive = on
	if was && !on {
		c.leave()
		c.tip.hide()
	}
	c.lock.Unlock()

	// Outside the lock: showing or hiding a widget refreshes it, and the layer
	// is Fyne's to refresh, not the chart's.
	if on {
		c.ptr.Show()
	} else {
		c.ptr.Hide()
	}
}

// Interactive reports whether a reader can hover, drag, zoom and click the
// chart. See [Interactive].
func (c *Chart) Interactive() bool {
	c.lock.Lock()
	defer c.lock.Unlock()
	return c.cfg.interactive
}

// MouseIn is called by Fyne. It is not part of the API.
func (p *pointer) MouseIn(ev *desktop.MouseEvent) {
	c := p.c
	c.lock.Lock()
	defer c.lock.Unlock()
	c.moved(ev.Position)
}

// MouseMoved is called by Fyne. It is not part of the API.
func (p *pointer) MouseMoved(ev *desktop.MouseEvent) {
	c := p.c
	c.lock.Lock()
	defer c.lock.Unlock()
	c.moved(ev.Position)
}

// MouseOut is called by Fyne. It is not part of the API.
func (p *pointer) MouseOut() {
	c := p.c
	c.lock.Lock()
	defer c.lock.Unlock()
	c.leave()
}

// leave is the pointer leaving the chart: whatever it was doing ends, and a
// band it was dragging out comes off the screen. The lock is held by the
// caller.
func (c *Chart) leave() {
	if c.in == nil {
		return
	}
	c.settle()
	c.dragged = false
	banded := c.banding
	c.endBand()
	leave := func() error {
		if err := c.in.Leave(); err != nil {
			return err
		}
		if banded {
			// Leave cancels a drag without drawing, so the band that was on
			// screen when the pointer left would stay there. Only then: a
			// pointer leaving a chart nobody was dragging has changed nothing,
			// and a frame drawn to prove it is a full render pass per exit.
			return c.live.Draw()
		}
		return nil
	}
	if err := c.target.Render(leave); err != nil {
		c.renderr = err
	}
	c.present()
	c.sharpen()
}

// MouseDown is called by Fyne. It is not part of the API.
func (p *pointer) MouseDown(ev *desktop.MouseEvent) {
	c := p.c
	c.lock.Lock()
	defer c.lock.Unlock()

	if c.in == nil || ev.Button != desktop.MouseButtonPrimary {
		return
	}
	c.drag, c.dragged = ev.Position, true
	if c.drags() {
		// A drag is about to start. Drawing it coarse is what keeps it
		// following the pointer; MouseUp and DragEnd put it back.
		c.coarsen()
	}
	if c.bands() && c.drags() {
		// The band is installed for the length of the drag and taken away
		// again on release, because an installed overlay makes every hover
		// redraw. See Chart.syncOverlay.
		c.banding = true
		c.syncOverlay()
	}
	c.down(ev.Position)
}

// down reports the press to figure, holding the surface. The lock is held by
// the caller.
func (c *Chart) down(pos fyne.Position) {
	press := func() error { return c.in.Down(float64(pos.X), float64(pos.Y)) }
	if err := c.target.Render(press); err != nil {
		c.renderr = err
	}
}

// up reports the release to figure and shows what it drew.
//
// It goes through the surface because a release is the one event that can draw
// several different things: a rubber band that zooms to itself, a click on a
// legend row that hides a layer, and — when the band has just been taken away —
// the frame that no longer has it in. The lock is held by the caller.
func (c *Chart) up(pos fyne.Position) {
	banded := c.banding
	// Before the release, so that nothing the release draws still carries the
	// band the reader has just let go of.
	c.endBand()
	release := func() error {
		if err := c.in.Up(float64(pos.X), float64(pos.Y)); err != nil {
			return err
		}
		if banded {
			// A selection draws nothing of its own — it answers a question —
			// so the band would still be on screen. A zoom to the band has
			// already drawn and this frame is identical to it, which paints
			// nothing.
			return c.live.Draw()
		}
		return nil
	}
	if err := c.target.Render(release); err != nil {
		c.renderr = err
		return
	}
	c.present()
}

// MouseUp is called by Fyne. It is not part of the API.
func (p *pointer) MouseUp(ev *desktop.MouseEvent) {
	c := p.c
	c.lock.Lock()
	defer c.lock.Unlock()

	if c.in == nil || ev.Button != desktop.MouseButtonPrimary {
		return
	}
	// A gesture that has ended has a last position nobody has drawn yet.
	c.settle()
	c.drag, c.dragged = ev.Position, false
	c.up(ev.Position)
	c.sharpen()
}

// Dragged is called by Fyne. It is not part of the API.
func (p *pointer) Dragged(ev *fyne.DragEvent) {
	c := p.c
	c.lock.Lock()
	defer c.lock.Unlock()

	if c.in == nil {
		return
	}
	c.drag, c.dragged = ev.Position, true
	c.moved(ev.Position)
}

// DragEnd is called by Fyne. It is not part of the API.
//
// Fyne reports the end of a drag without a position, so the last one seen is
// what the release is reported at — which is where the pointer is.
func (p *pointer) DragEnd() {
	c := p.c
	c.lock.Lock()
	defer c.lock.Unlock()

	if c.in == nil || !c.dragged {
		return
	}
	c.settle()
	c.dragged = false
	c.up(c.drag)
	c.sharpen()
}

// Scrolled is called by Fyne. It is not part of the API.
//
// Fyne counts a wheel notch in its own units and upwards; figure counts it in
// the browser's pixels and downwards, where a positive delta pushes the chart
// away and zooms out. [WheelScale] is the conversion.
func (p *pointer) Scrolled(ev *fyne.ScrollEvent) {
	c := p.c
	c.lock.Lock()
	defer c.lock.Unlock()

	if c.in == nil {
		return
	}
	if !c.steers() {
		// A chart that follows its data is not zoomed. See Chart.steers.
		return
	}
	delta := -float64(ev.Scrolled.DY) * c.cfg.wheel
	if delta == 0 {
		return
	}
	// Notches that arrive faster than they can be drawn are added up rather
	// than dropped: a wheel factor is exp(delta/1000), so applying the sum is
	// applying each of them in turn.
	c.wheel += delta
	c.wheelAt = ev.Position
	// A wheel has no end to report, so the coarse frames are timed out rather
	// than switched off by an event.
	c.coarsen()
	c.pace(c.zoom)
	c.armSharpen()
}

// zoom applies every wheel notch that has arrived since the last frame.
func (c *Chart) zoom() {
	delta := c.wheel
	c.wheel = 0
	if c.in == nil || delta == 0 {
		return
	}
	wheel := func() error { return c.in.Wheel(float64(c.wheelAt.X), float64(c.wheelAt.Y), delta) }
	if err := c.target.Render(wheel); err != nil {
		c.renderr = err
		return
	}
	c.present()
}

// DoubleTapped is called by Fyne. It is not part of the API.
func (p *pointer) DoubleTapped(*fyne.PointEvent) {
	c := p.c
	c.lock.Lock()
	defer c.lock.Unlock()

	if c.in == nil {
		return
	}
	c.steered = false
	c.sharpen()
	c.endBand()
	if err := c.target.Render(c.in.DoubleClick); err != nil {
		c.renderr = err
		return
	}
	c.present()
	c.viewChanged()
}

// Cursor is called by Fyne. It is not part of the API.
func (p *pointer) Cursor() desktop.Cursor { return p.c.cfg.cursor }

// moved is the one path a pointer position takes into the chart. Input decides
// whether it is a hover or a pan.
//
// A hover is not paced: it reads the hit index and draws nothing, so it costs
// microseconds and should feel immediate. A pan is paced, because it redraws —
// and dropping one loses nothing, since Input pans by the distance from the
// last position it was told about, so the next one covers the whole way.
//
// A press that has not yet moved past the click slop is not paced either. Those
// positions draw nothing — figure is still deciding whether this is a click —
// and dropping one would fold the slop into the first pan, moving the chart a
// few pixels further than the same drag would move it unpaced.
func (c *Chart) moved(pos fyne.Position) {
	if c.in == nil {
		return
	}
	if c.dragged && !c.drags() {
		// A chart that follows its data is not panned. See Chart.drags.
		return
	}
	if !c.dragged || !c.in.Dragging() {
		c.hover(pos)
		return
	}
	c.pace(func() { c.hover(pos) })
}

func (c *Chart) hover(pos fyne.Position) {
	if c.in == nil {
		return
	}
	// A move can pan, and a pan draws, so it holds the surface like any other
	// frame does.
	move := func() error {
		if err := c.in.Move(float64(pos.X), float64(pos.Y)); err != nil {
			return err
		}
		// A rubber band moves nothing, so figure draws nothing while one is
		// being dragged out: the feedback is the surface's, and this is it.
		if c.band() {
			return c.live.Draw()
		}
		return nil
	}
	if err := c.target.Render(move); err != nil {
		c.renderr = err
		return
	}
	c.present()
}
