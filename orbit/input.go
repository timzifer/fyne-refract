package orbit

import (
	"math"
	"slices"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver/desktop"
	"github.com/timzifer/figure/interact"
	"github.com/timzifer/figure/ir"
	"github.com/timzifer/figure/three"
)

// The pointer. A drag and a wheel are added up into the part of the turn no
// frame has shown yet, and a frame applies all of it at once — see pace.go —
// so a pointer that moves faster than the scene can be drawn loses nothing:
// three.Orbit adds angles and three.Dolly multiplies factors, and both are the
// same done once or in steps.

// hoverSlop is how far from a mark, in logical pixels, the pointer still
// finds it. A face is found anywhere inside it; this is for a line.
const hoverSlop = 6

// Dragged is called by Fyne. It is not part of the API.
func (c *Chart) Dragged(ev *fyne.DragEvent) {
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
	// Device y grows downward, so dragging down tips the scene toward the
	// reader: the camera's elevation falls, as it does in figure's own
	// example of the host's side of an orbit.
	c.dAz += float64(ev.Dragged.DX) * c.cfg.perPixel
	c.dEl -= float64(ev.Dragged.DY) * c.cfg.perPixel
	c.pace()
}

// DragEnd is called by Fyne. It is not part of the API.
func (c *Chart) DragEnd() {
	c.lock.Lock()
	defer c.lock.Unlock()

	// A drag that has stopped has a last step nobody has drawn yet.
	c.settle()
	c.dragging, c.turning = false, noView
}

// Scrolled is called by Fyne. It is not part of the API.
//
// Fyne counts a notch upward and in its own units. Upward brings the scene
// closer, which is a dolly by more than one.
func (c *Chart) Scrolled(ev *fyne.ScrollEvent) {
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
func (c *Chart) DoubleTapped(*fyne.PointEvent) {
	c.lock.Lock()
	defer c.lock.Unlock()
	c.home()
}

// MouseIn is called by Fyne. It is not part of the API.
func (c *Chart) MouseIn(ev *desktop.MouseEvent) {
	c.lock.Lock()
	defer c.lock.Unlock()
	c.hover(ev.Position)
}

// MouseMoved is called by Fyne. It is not part of the API.
func (c *Chart) MouseMoved(ev *desktop.MouseEvent) {
	c.lock.Lock()
	defer c.lock.Unlock()
	c.hover(ev.Position)
}

// MouseOut is called by Fyne. It is not part of the API.
func (c *Chart) MouseOut() {
	c.lock.Lock()
	defer c.lock.Unlock()
	if c.hovering && c.onHover != nil {
		c.hovering = false
		c.onHover(interact.Hit{Row: -1}, false)
	}
}

// Cursor is called by Fyne. It is not part of the API.
func (c *Chart) Cursor() desktop.Cursor { return c.cfg.cursor }

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
