package orbit

import (
	"time"

	"fyne.io/fyne/v2"
)

// Pacing is why a scene keeps up with a drag, for the reason package chart
// gives: Fyne's desktop driver delivers every pointer event the operating
// system queued, and drawing a frame for each of them asks for more work than
// there is time for.
//
// So a frame that would arrive before the last one has been paid for is not
// drawn. Nothing is lost by that here either — the turn it would have shown is
// still in the accumulators, and the next frame shows all of it — and what is
// left over when the pointer stops is drawn by a trailing timer.

// pace draws the waiting turn now if the last frame has been paid for, and
// otherwise leaves it for the timer.
func (c *Chart) pace() {
	if iv := c.interval(); iv == 0 || time.Since(c.lastFrame) >= iv {
		c.run()
		return
	}
	c.pending = true
	c.arm()
}

// run draws the waiting turn and records what it cost, which is what the next
// call measures itself against.
func (c *Chart) run() {
	c.pending = false
	start := time.Now()
	c.step()
	c.frameCost = time.Since(start)
	c.lastFrame = time.Now()
}

// interval is how long a frame has to have to itself: the last frame's own
// cost unless [FrameInterval] named one.
func (c *Chart) interval() time.Duration {
	if c.cfg.interval != 0 {
		if c.cfg.interval < 0 {
			return 0
		}
		return c.cfg.interval
	}
	return c.frameCost
}

// arm asks for the waiting turn to be drawn once the interval is up. The timer
// only ever posts to Fyne's goroutine; the work itself happens there.
func (c *Chart) arm() {
	if c.timer != nil {
		return
	}
	wait := c.interval() - time.Since(c.lastFrame)
	if wait < time.Millisecond {
		wait = time.Millisecond
	}
	c.timer = time.AfterFunc(wait, func() { fyne.Do(c.locked(c.flush)) })
}

// flush draws whatever the pacing held back. It runs on Fyne's goroutine.
func (c *Chart) flush() {
	c.timer = nil
	if !c.pending {
		return
	}
	if time.Since(c.lastFrame) < c.interval() {
		// Something else drew in the meantime and this is early again.
		c.arm()
		return
	}
	c.run()
}

// settle draws anything still waiting, now. It is what the end of a gesture
// calls, so that a scene does not land late.
func (c *Chart) settle() {
	if c.timer != nil {
		c.timer.Stop()
		c.timer = nil
	}
	if c.pending {
		c.run()
	}
}
