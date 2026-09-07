package chart

import (
	"time"

	"fyne.io/fyne/v2"
)

// Pacing is why a chart stays responsive under a drag.
//
// Fyne's desktop driver drains the whole operating-system event queue in one
// pass, so a pointer moving at a hundred events a second delivers a hundred of
// them — and each one that pans or zooms costs a frame, which on a chart of any
// size is tens of milliseconds. Drawing every one of them means asking for four
// times the work there is time for, and the backlog does not drain: the chart
// falls further behind the cursor for as long as the drag lasts.
//
// So a frame that would arrive before the last one has been paid for is not
// drawn. It is remembered instead, and the newest one wins — the reader is
// pointing somewhere now, and where they pointed two events ago is not
// interesting. What is left over is drawn by a trailing timer, so a drag that
// stops still ends up where it stopped.
//
// Nothing is lost by dropping a pan: refract's Input pans by the distance from
// the last position it was told about, so the next one it hears covers the
// whole way. Nothing is lost by dropping a zoom either, because the deltas are
// added up and applied together — a wheel factor is exp(delta/1000), and
// exp(a)*exp(b) is exp(a+b).

// doOnFyne puts work on Fyne's goroutine. It is the one place a timer crosses
// back, so it is the one place to look when that stops being true.
func doOnFyne(fn func()) { fyne.Do(fn) }

// pace runs fn now if the last frame has been paid for, and otherwise keeps it
// for later. It reports whether it ran.
func (c *Chart) pace(fn func()) bool {
	if iv := c.interval(); iv == 0 || time.Since(c.lastFrame) >= iv {
		c.run(fn)
		return true
	}
	c.pending = fn
	c.arm()
	return false
}

// run does the work and records what it cost, which is what the next call
// measures itself against.
func (c *Chart) run(fn func()) {
	start := time.Now()
	fn()
	c.frameCost = time.Since(start)
	c.lastFrame = time.Now()
}

// interval is how long a frame has to have to itself.
//
// It is the last frame's own cost unless [FrameInterval] named one: a chart
// that draws in three milliseconds should not be held to sixty a second, and
// one that takes fifty should not be asked for more than twenty. Measuring it
// is what makes the pacing follow the machine, the chart's size and the amount
// of data rather than a number someone guessed.
func (c *Chart) interval() time.Duration {
	if c.cfg.interval != 0 {
		if c.cfg.interval < 0 {
			return 0
		}
		return c.cfg.interval
	}
	return c.frameCost
}

// arm asks for the pending frame to be drawn once the interval is up. The
// timer only ever posts to Fyne's goroutine; the work itself happens there.
func (c *Chart) arm() {
	if c.timer != nil {
		return
	}
	wait := c.interval() - time.Since(c.lastFrame)
	if wait < time.Millisecond {
		wait = time.Millisecond
	}
	c.timer = time.AfterFunc(wait, func() { doOnFyne(c.locked(c.flush)) })
}

// flush draws whatever the pacing held back. It runs on Fyne's goroutine.
func (c *Chart) flush() {
	c.timer = nil
	fn := c.pending
	c.pending = nil
	if fn == nil {
		return
	}
	if time.Since(c.lastFrame) < c.interval() {
		// Something else drew in the meantime and this is early again.
		c.pending = fn
		c.arm()
		return
	}
	c.run(fn)
}

// settle draws anything still pending, now.
//
// It is what the end of a gesture calls: a drag that stopped moving has one
// last position nobody has drawn yet, and waiting out a timer to show it is a
// chart that lands late.
func (c *Chart) settle() {
	if c.timer != nil {
		c.timer.Stop()
		c.timer = nil
	}
	fn := c.pending
	c.pending = nil
	if fn != nil {
		c.run(fn)
	}
}
