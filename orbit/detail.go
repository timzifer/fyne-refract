package orbit

import (
	"time"

	"fyne.io/fyne/v2"
)

// Detail is package chart's level-of-detail trade, made for the chart nobody is
// touching.
//
// Several orbit widgets side by side that a program links — a camera turned in
// one turning the others, say — draw every one of them on every frame of the
// gesture, one after another on Fyne's goroutine. The chart under the pointer
// is the one the reader is watching; the others are peripheral vision, and
// what they need is to keep up rather than to be sharp. So a program can have
// those drawn coarser than the screen while the gesture lasts
// ([Chart.SetCoarse]) and sharp again when it ends, and it learns when a
// gesture starts and ends from [Chart.OnGesture].
//
// It pays for the reason it does in package chart: the rasterizer's cost is per
// pixel, and Fyne stretches a smaller frame on the graphics card for nothing.
// Half the resolution is a quarter of the pixels.
//
// None of it is automatic, because which charts are "the others" is not the
// widget's to know — it does not know what it has been linked to.

// wheelSettles is how long after the last wheel notch a wheel gesture counts as
// over. A drag says when it ends; a wheel does not, so it is timed. It is
// package chart's number for the same question.
const wheelSettles = 120 * time.Millisecond

// SetCoarse draws the chart at the resolution [Detail] names, or back at the
// screen's own.
//
// Going coarse draws nothing new on screen: the frame showing stays until the
// next one, which is the one a camera moved for. Going sharp redraws at once,
// so that the chart a reader stops on is never left blurred.
//
// It does nothing on a chart whose [Detail] is not below one.
func (c *Chart) SetCoarse(on bool) {
	c.lock.Lock()
	defer c.lock.Unlock()
	if on == c.coarse || (on && !c.coarsens()) {
		return
	}
	c.coarse = on
	if c.live == nil || c.dpr <= 0 {
		// Nothing has been rasterized yet; the first frame reads coarse itself.
		return
	}
	if err := c.target.Render(func() error { return c.live.Rescale(c.rasterScale()) }); err != nil {
		c.renderr = err
		return
	}
	c.renderr = nil
	if !on {
		c.target.Present()
	}
}

// Coarse reports whether the chart is drawn below the screen's resolution.
func (c *Chart) Coarse() bool {
	c.lock.Lock()
	defer c.lock.Unlock()
	return c.coarse
}

// OnGesture registers what to call when a reader starts turning or dollying
// the chart, and once more with false when they stop. Passing nil removes it.
//
// It is the other half of [Chart.SetCoarse], and the link is one line:
//
//	a.OnGesture(func(active bool) { b.SetCoarse(active) })
//
// A drag starts at its first movement and stops when it is let go. A wheel has
// no end to report, so it stops once the wheel has been still for a moment.
//
// The handler runs on Fyne's goroutine with this chart held, so it must not
// call back into this chart; calling into another is what it is for.
func (c *Chart) OnGesture(fn func(active bool)) {
	c.lock.Lock()
	defer c.lock.Unlock()
	c.onGesture = fn
}

// coarsens reports whether [Detail] asked for anything below the screen.
func (c *Chart) coarsens() bool { return c.cfg.detail > 0 && c.cfg.detail < 1 }

// rasterScale is the device pixel ratio frames are rasterized at: the screen's,
// or the part of it [Detail] names while the chart is coarse.
func (c *Chart) rasterScale() float64 {
	if c.coarse {
		return c.dpr * float64(c.cfg.detail)
	}
	return c.dpr
}

// gestureBegins reports a gesture starting, once however many events it takes.
// The lock is held by the caller.
func (c *Chart) gestureBegins() {
	if c.gesturing {
		return
	}
	c.gesturing = true
	if c.onGesture != nil {
		c.onGesture(true)
	}
}

// gestureEnds reports the gesture over, if one was reported begun. The lock is
// held by the caller.
func (c *Chart) gestureEnds() {
	if c.wheelTimer != nil {
		c.wheelTimer.Stop()
		c.wheelTimer = nil
	}
	if !c.gesturing {
		return
	}
	c.gesturing = false
	if c.onGesture != nil {
		c.onGesture(false)
	}
}

// armWheelEnd ends a wheel gesture once the wheel has been still for
// [wheelSettles]. The lock is held by the caller.
func (c *Chart) armWheelEnd() {
	if c.wheelTimer != nil {
		c.wheelTimer.Reset(wheelSettles)
		return
	}
	c.wheelTimer = time.AfterFunc(wheelSettles, func() { fyne.Do(c.locked(c.wheelStopped)) })
}

// wheelStopped is the wheel's end. It runs on Fyne's goroutine.
func (c *Chart) wheelStopped() {
	c.wheelTimer = nil
	if c.dragging {
		// A drag took the gesture over, and its release is its end.
		return
	}
	c.settle()
	c.gestureEnds()
}
