package chart

import (
	"time"

	"fyne.io/fyne/v2"
	"github.com/timzifer/figure"
	"github.com/timzifer/figure/data"
)

// A transition is figure's — a keyed join between two tables, blended in data
// space — and the clock is the host's. [figure.Transition.Advance] is told
// what time it is and says whether there is more to come; what the widget adds
// is a frame timer that keeps telling it, on Fyne's goroutine, until there is
// not.

// DefaultFrameRate is how often a transition is advanced when [Chart.Animate]
// is not already pacing the chart: about sixty frames a second, which is what
// a display shows and what a hand moving something across one expects.
const DefaultFrameRate = 16 * time.Millisecond

// Transition builds a transition from this chart's rows to another set of
// them, ready to be driven.
//
// It is [figure.Live.Transition] on the chart's own [figure.Live], and the
// value it returns is figure's, so the whole of that vocabulary chains onto
// it before it is handed back to [Chart.Play]:
//
//	tw, err := data.NewTween(before, after, "id")
//	tr, err := c.Transition(tw)
//	c.Play(tr.Over(400 * time.Millisecond).Ease(figure.EaseOut))
//
// It draws nothing. A chart that has not been laid out yet has no Live and
// returns nil, nil — there is nothing to transition between until there is a
// chart on screen.
func (c *Chart) Transition(tweens ...*data.Tween) (*figure.Transition, error) {
	c.lock.Lock()
	defer c.lock.Unlock()
	if c.live == nil {
		return nil, nil
	}
	return c.live.Transition(tweens...)
}

// Play drives a transition to its end and returns the function that stops it
// early. A nil transition, or one already finished, plays nothing.
//
// done, if it is not nil, is called once the transition has reached its end —
// on Fyne's goroutine, with the chart free, so a handler may draw, swap the
// data or start the next transition. It is not called when the transition is
// stopped early: stopping is the caller saying they know where it got to.
//
// A transition being played belongs to the goroutine playing it. Nothing on it
// — not even [figure.Transition.Done] — may be read from elsewhere while it
// runs, which is what done exists for.
//
// Each frame advances the transition by the wall clock and draws, on Fyne's
// goroutine and holding the surface, so a transition and the painter are never
// in the buffer at once. The clock rather than a frame count is what makes a
// four-hundred-millisecond transition take four hundred milliseconds on a
// machine that cannot draw sixty frames in that time: it skips the frames it
// cannot afford instead of running long.
//
// Stopping leaves the chart wherever the transition had reached.
// [figure.Transition.Finish] jumps to the end instead, and a caller who wants
// that on interruption calls it after stopping. [Chart.Close] stops a playing
// transition, and so does playing another one.
func (c *Chart) Play(tr *figure.Transition, done func()) (stop func()) {
	c.lock.Lock()
	defer c.lock.Unlock()

	if c.transFn != nil {
		c.transFn()
		c.transFn = nil
	}
	if tr == nil || tr.Done() || c.live == nil {
		return func() {}
	}

	stopped := make(chan struct{})
	var once bool
	stop = func() {
		if !once {
			once = true
			close(stopped)
		}
	}
	c.transFn = stop

	go func() {
		t := time.NewTicker(c.frameRate())
		defer t.Stop()
		for {
			select {
			case <-stopped:
				return
			case now := <-t.C:
				running := make(chan bool, 1)
				fyne.Do(c.locked(func() { running <- c.advance(tr, now) }))
				select {
				case <-stopped:
					return
				case more := <-running:
					if more {
						continue
					}
					stop()
					if done != nil {
						// On Fyne's goroutine and outside the chart's lock, so
						// that a handler may call back into the chart it was
						// told about.
						fyne.Do(done)
					}
					return
				}
			}
		}
	}()
	return stop
}

// frameRate is how often a transition is advanced. A chart that was given a
// frame interval is paced at it — asking for frames faster than the chart can
// draw them is what the pacing exists to stop — and one that was not is
// advanced at [DefaultFrameRate]. The lock is held by the caller.
func (c *Chart) frameRate() time.Duration {
	if c.cfg.interval > 0 && c.cfg.interval > DefaultFrameRate {
		return c.cfg.interval
	}
	return DefaultFrameRate
}

// advance puts the transition where the clock says it is and shows the frame,
// reporting whether there is more to come. It runs on Fyne's goroutine with
// the lock held.
func (c *Chart) advance(tr *figure.Transition, now time.Time) bool {
	if c.live == nil {
		return false
	}
	var running bool
	step := func() error {
		var err error
		running, err = tr.Advance(now)
		return err
	}
	if err := c.target.Render(step); err != nil {
		c.renderr = err
		return false
	}
	c.present()
	// A transition with [figure.Transition.Rescale] on moves the axes, which
	// is a change of view a linked chart is owed. Whether this one rescales is
	// the transition's own business and not readable from here, so the news
	// goes out on every frame — which costs nothing at all unless somebody
	// registered [Chart.OnViewChange], and is exactly what they wanted if
	// they did.
	c.viewChanged()
	return running
}
