package chart

import (
	"time"

	"fyne.io/fyne/v2"
	"github.com/timzifer/figure/data"
)

// Stream tells the chart which stream to freeze before each frame.
//
// A [data.Stream] is not a source and cannot be drawn from directly: a stream
// being appended to between two column reads is a stream that disagrees with
// itself. Snapshot copies the live rows into the buffer the renderer is not
// reading and swaps the two, and it belongs between frames rather than during
// one — so the chart does it, once, immediately before it draws.
//
//	st := data.NewStream("t", "y").Window(600)
//	p.Add(geom.Line(st.Source(), geom.X("t"), geom.Y("y")))
//	c := chart.New(p)
//	c.Stream(st)
//
// Appending may happen on any goroutine. Asking for the frame that shows it is
// [Chart.Redraw], or [Chart.Animate] for a chart that should keep moving.
func (c *Chart) Stream(s *data.Stream) {
	c.lock.Lock()
	defer c.lock.Unlock()
	c.stream = s
}

// Animate redraws the chart on a timer and returns the function that stops it.
//
// It is the honest shape for a live chart: the producer appends whenever it
// has something, and the screen is repainted at a rate a screen can show.
// Each tick draws on Fyne's goroutine, and a tick whose frame is identical to
// the last paints nothing — so an idle chart on a ticker costs a comparison.
//
// Calling it again replaces the previous timer. [Chart.Close] stops it too.
func (c *Chart) Animate(every time.Duration) (stop func()) {
	c.lock.Lock()
	defer c.lock.Unlock()
	if c.stopFn != nil {
		c.stopFn()
		c.stopFn = nil
	}
	if every <= 0 {
		return func() {}
	}

	done := make(chan struct{})
	var once bool
	stop = func() {
		if !once {
			once = true
			close(done)
		}
	}
	c.stopFn = stop

	go func() {
		t := time.NewTicker(every)
		defer t.Stop()
		for {
			select {
			case <-done:
				return
			case <-t.C:
				fyne.Do(c.locked(c.draw))
			}
		}
	}()
	return stop
}
