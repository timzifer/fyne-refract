package orbit

import (
	"math"
	"slices"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/widget"
	"github.com/timzifer/figure/interact"
	"github.com/timzifer/figure/ir"
	"github.com/timzifer/figure/three"
)

// The pointer. A drag and a wheel are added up into the part of the turn no
// frame has shown yet, and a frame applies all of it at once — see pace.go —
// so a pointer that moves faster than the scene can be drawn loses nothing:
// three.Orbit adds angles and three.Dolly multiplies factors, and both are the
// same done once or in steps.
//
// None of it is the chart's own. The handlers belong to pointer, a layer laid
// over the raster that is hidden unless the chart was made [Interactive], for
// the reason package chart gives: Fyne sends an event to whatever implements
// the interface for it, wanted or not, and a hidden layer is not found at all.

// hoverSlop is how far from a mark, in logical pixels, the pointer still
// finds it. A face is found anywhere inside it; this is for a line.
const hoverSlop = 6

// pointer is the layer that takes the pointer for a chart.
//
// It became Mouseable when [Select] arrived. A drag arrives through Dragged, so
// before that a press that did not move turned nothing and had nothing to say;
// now it picks a row, and the press and the release are what tell a click from
// the beginning of a turn. It is Mouseable rather than Tappable for the reason
// package chart gives: a click already arrives here, and Tappable would deliver
// it a second time.
type pointer struct {
	widget.BaseWidget
	c *Chart
}

var (
	_ fyne.Widget         = (*pointer)(nil)
	_ fyne.Draggable      = (*pointer)(nil)
	_ fyne.Scrollable     = (*pointer)(nil)
	_ fyne.DoubleTappable = (*pointer)(nil)
	_ desktop.Hoverable   = (*pointer)(nil)
	_ desktop.Mouseable   = (*pointer)(nil)
	_ desktop.Cursorable  = (*pointer)(nil)
)

// clickSlop is how far a press may travel and still be a click rather than the
// beginning of a turn, in logical pixels. Fyne has its own, larger slop before
// it calls a movement a drag; this one exists so that the wobble of a firm
// click on a trackpad does not pick a row and then turn the scene away from it.
const clickSlop = 3

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

// SetInteractive lets a reader turn the scene, or takes the pointer back from
// them. It is [Interactive] for a chart already on screen.
//
// Taking it back lands a drag in progress where it had got to and ends a
// hover; the cameras stay where the reader put them. [Chart.Home] is the way
// back from there.
func (c *Chart) SetInteractive(on bool) {
	c.lock.Lock()
	was := c.cfg.interactive
	c.cfg.interactive = on
	if was && !on {
		c.leave()
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

// Interactive reports whether a reader can turn, dolly and hover the scene.
// See [Interactive].
func (c *Chart) Interactive() bool {
	c.lock.Lock()
	defer c.lock.Unlock()
	return c.cfg.interactive
}

// leave ends whatever the pointer was doing: the turn it built up is drawn,
// the gesture forgotten, and a hover reported over. The lock is held by the
// caller.
func (c *Chart) leave() {
	c.settle()
	c.dragging, c.turning = false, noView
	if c.hovering && c.onHover != nil {
		c.hovering = false
		c.onHover(interact.Hit{Row: -1}, false)
	}
}

// Dragged is called by Fyne. It is not part of the API.
func (p *pointer) Dragged(ev *fyne.DragEvent) {
	c := p.c
	c.lock.Lock()
	defer c.lock.Unlock()

	if c.live == nil {
		return
	}
	if !c.dragging {
		// Fyne reports a drag once it has moved past its own slop, so the
		// press it began with is where this event is less how far it came.
		c.dragging = true
		c.turning = c.viewAt(ev.Position.Subtract(ev.Dragged))
	}
	if c.turning == noView {
		return
	}
	// A drag takes hold of the scene: the side facing the reader follows the
	// pointer, as it does in three.js's OrbitControls, in Blender and in
	// matplotlib. That is the camera going the other way round — a drag to
	// the right carries it left about the up axis, so its azimuth falls, and
	// device y grows downward, so a drag down lifts it and its elevation
	// rises.
	c.dAz -= float64(ev.Dragged.DX) * c.cfg.perPixel
	c.dEl += float64(ev.Dragged.DY) * c.cfg.perPixel
	c.pace()
}

// DragEnd is called by Fyne. It is not part of the API.
func (p *pointer) DragEnd() {
	c := p.c
	c.lock.Lock()
	defer c.lock.Unlock()

	// A drag that has stopped has a last step nobody has drawn yet.
	c.settle()
	c.dragging, c.turning = false, noView
	// The release that follows is the end of a turn rather than a click, and
	// Fyne delivers DragEnd before it.
	c.pressed = false
}

// MouseDown is called by Fyne. It is not part of the API.
func (p *pointer) MouseDown(ev *desktop.MouseEvent) {
	c := p.c
	c.lock.Lock()
	defer c.lock.Unlock()

	if ev.Button != desktop.MouseButtonPrimary {
		return
	}
	c.down, c.pressed = ev.Position, true
}

// MouseUp is called by Fyne. It is not part of the API.
//
// A release that neither turned the scene nor travelled is a click, and a click
// picks a row. Everything else is the end of a gesture DragEnd has already
// settled.
func (p *pointer) MouseUp(ev *desktop.MouseEvent) {
	c := p.c
	c.lock.Lock()
	defer c.lock.Unlock()

	pressed := c.pressed
	c.pressed = false
	if ev.Button != desktop.MouseButtonPrimary || !pressed || c.dragging {
		return
	}
	if dx, dy := ev.Position.X-c.down.X, ev.Position.Y-c.down.Y; dx*dx+dy*dy > clickSlop*clickSlop {
		return
	}
	c.clicked(ev.Position)
}

// Scrolled is called by Fyne. It is not part of the API.
//
// Fyne counts a notch upward and in its own units. Upward brings the scene
// closer, which is a dolly by more than one.
func (p *pointer) Scrolled(ev *fyne.ScrollEvent) {
	c := p.c
	c.lock.Lock()
	defer c.lock.Unlock()

	if c.live == nil {
		return
	}
	delta := float64(ev.Scrolled.DY) * c.cfg.wheel
	if delta == 0 {
		return
	}
	view := c.viewAt(ev.Position)
	if view == noView {
		return
	}
	if c.wheel != 0 && view != c.wheelView {
		// Notches still waiting belong to the view they were turned over.
		c.settle()
	}
	c.wheel += delta
	c.wheelView = view
	c.pace()
}

// DoubleTapped is called by Fyne. It is not part of the API.
func (p *pointer) DoubleTapped(*fyne.PointEvent) {
	c := p.c
	c.lock.Lock()
	defer c.lock.Unlock()
	c.home()
}

// MouseIn is called by Fyne. It is not part of the API.
func (p *pointer) MouseIn(ev *desktop.MouseEvent) {
	c := p.c
	c.lock.Lock()
	defer c.lock.Unlock()
	c.hover(ev.Position)
}

// MouseMoved is called by Fyne. It is not part of the API.
func (p *pointer) MouseMoved(ev *desktop.MouseEvent) {
	c := p.c
	c.lock.Lock()
	defer c.lock.Unlock()
	c.hover(ev.Position)
}

// MouseOut is called by Fyne. It is not part of the API.
func (p *pointer) MouseOut() {
	c := p.c
	c.lock.Lock()
	defer c.lock.Unlock()
	if c.hovering && c.onHover != nil {
		c.hovering = false
		c.onHover(interact.Hit{Row: -1}, false)
	}
}

// Cursor is called by Fyne. It is not part of the API.
func (p *pointer) Cursor() desktop.Cursor { return p.c.cfg.cursor }

// hover asks the last frame's hit index what is under the pointer. It draws
// nothing, so it is not paced, and it says nothing when a miss follows a miss.
func (c *Chart) hover(pos fyne.Position) {
	if c.onHover == nil || c.live == nil {
		return
	}
	h, found := c.live.Index().At(ir.Point{X: pos.X, Y: pos.Y}, hoverSlop)
	if !found && !c.hovering {
		return
	}
	c.hovering = found
	c.onHover(h, found)
}

// viewAt is which views a gesture at pos turns.
//
// A figure with one view is that view wherever the gesture starts — over its
// title too — because there is nothing else it could mean.
func (c *Chart) viewAt(pos fyne.Position) int {
	if c.cfg.together {
		return everyView
	}
	if i := c.live.ViewAt(float64(pos.X), float64(pos.Y)); i >= 0 {
		return i
	}
	if c.live.ViewCount() == 1 {
		return 0
	}
	return noView
}

// step applies whatever turn has built up since the last frame, draws it and
// reports it. It is what pace runs.
func (c *Chart) step() {
	if c.live == nil {
		return
	}
	if dAz, dEl := c.dAz, c.dEl; dAz != 0 || dEl != 0 {
		c.dAz, c.dEl = 0, 0
		c.turn(c.turning, func(cam three.Camera) three.Camera { return three.Orbit(cam, dAz, dEl) })
	}
	if wheel := c.wheel; wheel != 0 {
		c.wheel = 0
		by := math.Exp(wheel / 1000)
		c.turn(c.wheelView, func(cam three.Camera) three.Camera { return three.Dolly(cam, by) })
	}
	if len(c.touched) == 0 {
		return
	}
	c.draw()
	c.report()
}

// turn moves the camera of one view, or of every view, by fn.
//
// Every view is moved by the same amount rather than pointed at the same
// camera: a front / top / side figure turned together stays a front, a top and
// a side, turned.
func (c *Chart) turn(view int, fn func(three.Camera) three.Camera) {
	switch {
	case view == everyView:
		for i := range c.live.ViewCount() {
			c.live.SetCamera(i, fn(c.live.CameraOf(i)))
			c.touch(i)
		}
	case view >= 0:
		c.live.SetCamera(view, fn(c.live.CameraOf(view)))
		c.touch(view)
	}
}

func (c *Chart) touch(view int) {
	if !slices.Contains(c.touched, view) {
		c.touched = append(c.touched, view)
	}
}

// report tells OnCamera about every view a reader moved in the frame just
// drawn.
func (c *Chart) report() {
	if c.onCamera != nil {
		for _, i := range c.touched {
			c.onCamera(i, c.live.CameraOf(i))
		}
	}
	c.touched = c.touched[:0]
}

// stopGesture forgets a turn that has not been drawn and the gesture it came
// from.
func (c *Chart) stopGesture() {
	c.pending = false
	c.dragging, c.turning = false, noView
	c.dAz, c.dEl, c.wheel = 0, 0, 0
	c.wheelView = noView
	c.touched = c.touched[:0]
}
