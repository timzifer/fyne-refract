package chart

import (
	"image"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/widget"
	"github.com/timzifer/figure"
	"github.com/timzifer/figure/data"
	"github.com/timzifer/figure/scale"
	fynefigure "github.com/timzifer/fyne_figure"
	"github.com/timzifer/fyne_figure/internal/look"
)

// Chart is a figure plot as a Fyne widget.
//
// It is drawn by [fynefigure.Target] and steered by [figure.Input], so what
// it shows is what the same plot would write to a file, and how it behaves is
// how the same plot behaves in a browser or a native window: hover to see what
// is under the pointer, drag to pan, turn the wheel to zoom about it, double
// click to go back to the whole picture.
//
// It behaves that way once it is [Interactive], and not before. Until then it
// takes no pointer events at all and is a picture that follows its size, its
// theme and its data.
//
// The chart is opened at the first layout, because a plot needs a size and a
// widget has none until it is laid out. Everything before that — adding
// layers, setting scales — belongs on the plot.
type Chart struct {
	widget.BaseWidget

	plot *figure.Plot
	cfg  config

	target *fynefigure.Target
	live   *figure.Live
	in     *figure.Input

	// w and h are the logical size the chart was last laid out at, and dpr the
	// device pixel ratio it was last rasterized at.
	w, h int
	dpr  float64

	// lock serialises everything the chart does. Fyne's events arrive on one
	// goroutine and the pacing and sharpening timers arrive on another — on a
	// desktop driver fyne.Do puts those back on the first, and under the test
	// driver it runs them where they are. Holding this rather than depending
	// on which is what makes the widget correct either way.
	lock sync.Mutex

	// painterPx is the pixel geometry the painter last asked for when it
	// disagreed with what had been rasterized. It is written from the painter
	// goroutine, which is why it is behind a lock.
	mu        sync.Mutex
	painterPx image.Point

	drag    fyne.Position
	dragged bool

	// Pacing. lastFrame and frameCost are when the last frame was drawn and
	// what it cost; pending is a frame that arrived too soon, and timer is
	// what will draw it. See pace.go.
	lastFrame time.Time
	frameCost time.Duration
	pending   func()
	timer     *time.Timer

	// wheel is zoom that arrived faster than it could be drawn, waiting to be
	// applied in one go.
	wheel   float64
	wheelAt fyne.Position

	// coarse says the rasterizer is at the interactive resolution rather than
	// the screen's, and sharpTimer is what puts it back after a wheel. See
	// detail.go. coarse is read from the painter's goroutine, so it lives
	// under the same lock as painterPx.
	coarse     bool
	sharpTimer *time.Timer

	// steered records that a reader has zoomed or panned, which is what stops
	// a followed axis from taking their view away again on the next frame.
	steered bool

	// unniced records that the followed axes have been checked for the framing
	// that would make them move in steps. It is a once, not a flag: the check
	// needs a drawn frame to reach the scales, and it costs a rebuild.
	unniced bool

	tip *tooltip

	// ptr is the layer that takes the pointer. It is always in the widget's
	// tree and hidden unless the chart is [Interactive]; see input.go.
	ptr *pointer

	// The overlay layer. overlay is what a caller installed and brush is the
	// rubber band of a [DragMode] drag; ov composes the two and is what figure
	// is actually given. banding says a band is being dragged out right now,
	// which is the only time the brush paints — an installed overlay makes
	// every hover redraw (see [figure.Input.Move]), so a chart with neither
	// has none installed and pays nothing.
	overlay figure.Overlay
	brush   *figure.Brush
	ov      *overlays
	banding bool
	onView  func(figure.View)

	stream *data.Stream
	stopFn func()

	// transFn stops the transition being driven, if there is one. It is not
	// stopFn: a live chart on a ticker and a transition over it are two
	// animations, and starting one must not silently end the other.
	transFn func()

	// themed remembers what the chart was last built for, so that a settings
	// change that touched neither is not a rebuild.
	themed look.State

	// hooked records that the plot carries this chart's event handlers. They
	// belong to the plot rather than to the chart, so they outlive a chart
	// rebuilt for a new typeface and must not be added twice.
	hooked  bool
	renderr error
}

// A chart is a widget and nothing else. The pointer interfaces are its
// pointer layer's, which is only there to be found once the chart is
// [Interactive] — see input.go for why the widget must not carry them itself.
var _ fyne.Widget = (*Chart)(nil)

// New returns a widget showing p.
//
// Nothing is rasterized until the widget is laid out, so a chart built and
// never shown has taken no memory beyond the plot itself.
func New(p *figure.Plot, opts ...Option) *Chart {
	c := &Chart{plot: p, cfg: defaults(), dpr: 1}
	for _, o := range opts {
		o(&c.cfg)
	}
	c.overlay, c.ov = c.cfg.overlay, &overlays{}
	c.brush = c.cfg.brush
	if !c.cfg.brushSet {
		// The default band: figure's own look, which reads against either
		// theme because it is drawn in the theme's own axis colour.
		c.brush = &figure.Brush{}
	}
	c.ptr = newPointer(c, c.cfg.interactive)
	c.ExtendBaseWidget(c)
	return c
}

// Plot returns the plot the chart shows. Changing it — adding a layer,
// replacing a scale — takes effect on the next [Chart.Rebuild].
func (c *Chart) Plot() *figure.Plot { return c.plot }

// Live returns the chart being drawn, or nil before the first layout.
//
// It is the whole of figure's interactive API: zoom to a rectangle, read the
// hit index, ask what size the surface is. A caller that drives it directly
// should call [Chart.Present] afterwards, or simply [Chart.Refresh].
func (c *Chart) Live() *figure.Live { return c.live }

// Target returns what the chart is drawn into, or nil before the first
// layout. It is the way to the pixels: an export of exactly what is on screen
// reads [fynefigure.Target.Image].
func (c *Chart) Target() *fynefigure.Target { return c.target }

// Err reports what went wrong in the last frame, if anything.
//
// A widget has nowhere to return an error to — Fyne's layout and event
// callbacks return nothing — so a failed frame is kept here and the widget
// shows the frame before it.
func (c *Chart) Err() error { return c.renderr }

// Refresh redraws the chart. It is Fyne's own name for "show what has
// changed", and it is what to call from a button, a menu or any other place
// that already runs on Fyne's goroutine.
func (c *Chart) Refresh() { c.BaseWidget.Refresh() }

// Redraw asks for a frame from anywhere.
//
// It is [Chart.Refresh] for a goroutine of your own: a producer appending to a
// stream, a ticker, a network read. The frame is drawn on Fyne's goroutine in
// its turn, which is what keeps the rasterizer and the painter off each
// other's pixels.
func (c *Chart) Redraw() { fyne.Do(c.locked(c.draw)) }

// locked wraps an operation so that it holds the chart while it runs. It is
// what a timer or another goroutine posts, since those do not arrive on the
// goroutine Fyne's events do.
func (c *Chart) locked(fn func()) func() {
	return func() {
		c.lock.Lock()
		defer c.lock.Unlock()
		fn()
	}
}

// Rebuild resolves the plot again — after a layer was added, a scale replaced,
// a facet changed — and redraws.
//
// Like any fresh start it forgets where the chart was zoomed to. A chart whose
// data changed but whose shape did not wants [Chart.Refresh] instead.
func (c *Chart) Rebuild() error {
	c.lock.Lock()
	defer c.lock.Unlock()
	if c.live == nil {
		return nil
	}
	if err := c.live.Rebuild(); err != nil {
		return err
	}
	c.draw()
	return c.renderr
}

// Autoscale releases every zoom and pan and redraws. It is what a double click
// does.
func (c *Chart) Autoscale() error {
	c.lock.Lock()
	defer c.lock.Unlock()
	if c.live == nil {
		return nil
	}
	c.steered = false
	if err := c.target.Render(c.live.Autoscale); err != nil {
		return err
	}
	c.present()
	c.viewChanged()
	return nil
}

// Close releases the chart's pixels and stops anything animating it.
//
// A widget removed from its tree is closed for you when Fyne destroys its
// renderer; this is for a caller who wants it gone sooner.
func (c *Chart) Close() error {
	c.lock.Lock()
	defer c.lock.Unlock()
	return c.close()
}

// close is Close with the lock already held.
func (c *Chart) close() error {
	if c.stopFn != nil {
		c.stopFn()
		c.stopFn = nil
	}
	if c.transFn != nil {
		c.transFn()
		c.transFn = nil
	}
	if c.timer != nil {
		c.timer.Stop()
		c.timer = nil
	}
	if c.sharpTimer != nil {
		c.sharpTimer.Stop()
		c.sharpTimer = nil
	}
	c.pending = nil
	var err error
	if c.live != nil {
		err = c.live.Close()
		c.live, c.in = nil, nil
	}
	if c.target != nil {
		if cerr := c.target.Close(); err == nil {
			err = cerr
		}
		c.target = nil
	}
	c.tip.close()
	return err
}

// CreateRenderer is called by Fyne. It is not part of the API.
func (c *Chart) CreateRenderer() fyne.WidgetRenderer {
	c.lock.Lock()
	defer c.lock.Unlock()
	c.ExtendBaseWidget(c)
	c.ensureTarget()
	c.tip = newTooltip(c)
	objects := append([]fyne.CanvasObject{c.target.Object(), c.ptr}, c.tip.objects()...)
	return &renderer{c: c, objects: objects}
}

// resize lays the chart out at a new size, which is where a Live is born.
func (c *Chart) resize(size fyne.Size) {
	w, h := int(size.Width), int(size.Height)
	if w <= 0 || h <= 0 {
		return
	}
	c.ensureTarget()

	if c.live == nil {
		c.applyTheme()
		// Not inside Render: Plot.Live opens the target, which takes the
		// surface for itself.
		live, err := c.plot.Live(c.target)
		if err != nil {
			c.renderr = err
			return
		}
		c.renderr = nil
		c.live = live.TrackRows(c.tracksRows())
		c.in = c.live.Input().Drag(c.cfg.drag)
		c.endBand()
		c.hookEvents()
		c.w, c.h = c.plot.Size()
	}

	if w != c.w || h != c.h {
		if err := c.target.Render(func() error { return c.live.Resize(w, h) }); err != nil {
			c.renderr = err
			return
		}
		c.w, c.h = w, h
	}
	c.checkScale()
	c.draw()
}

// ensureTarget builds the render target, in the typeface the theme asks for.
func (c *Chart) ensureTarget() {
	if c.target != nil {
		return
	}
	var opts []fynefigure.Option
	if c.cfg.font {
		if regular, bold, italic, ok := c.themeFonts(); ok {
			opts = append(opts, fynefigure.Font(regular, bold, italic))
		}
	}
	c.target = fynefigure.New(opts...)
	c.target.OnGeometry(c.painterGeometry)
	c.themed = c.themeStateNow()
}

// painterGeometry is called from Fyne's painter when it is about to draw the
// chart at a pixel size the chart was not rasterized at — a window moved to a
// display with a different device pixel ratio. It records the size and does
// nothing else: it is the painter's goroutine, which is no place to rasterize
// a chart, and it does not have to be. A change of scale makes Fyne refresh
// its content, and the refresh is where [Chart.checkScale] reads this.
func (c *Chart) painterGeometry(widthPx, heightPx int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	// While the chart is drawn coarse the frame is meant to be smaller than
	// what the painter asked for, so the difference says nothing about the
	// display and reading it as a device pixel ratio would undo the coarsening
	// on the next refresh.
	if c.coarse {
		return
	}
	c.painterPx = image.Pt(widthPx, heightPx)
}

// checkScale rasterizes at the device pixel ratio the surroundings now have,
// if that is not the one the last frame was drawn at.
func (c *Chart) checkScale() {
	if c.live == nil {
		return
	}
	dpr := c.scaleFactor()
	if dpr == c.dpr {
		return
	}
	if err := c.target.Render(func() error { return c.live.Rescale(dpr) }); err != nil {
		c.renderr = err
		return
	}
	c.dpr = dpr
}

// scaleFactor is the device pixel ratio the chart should rasterize at.
//
// The painter is the authority — it says how many physical pixels the widget
// covers — but it has only spoken once it has painted a frame. Until then the
// canvas's own scale is the best answer there is.
func (c *Chart) scaleFactor() float64 {
	c.mu.Lock()
	asked := c.painterPx
	c.painterPx = image.Point{}
	c.mu.Unlock()

	if asked.X > 0 && c.w > 0 {
		return float64(asked.X) / float64(c.w)
	}
	if app := fyne.CurrentApp(); app != nil {
		if drv := app.Driver(); drv != nil {
			if cv := drv.CanvasForObject(c); cv != nil {
				if s := float64(cv.Scale()); s > 0 {
					return s
				}
			}
		}
	}
	if c.dpr > 0 {
		return c.dpr
	}
	return 1
}

// draw paints a frame. It runs on Fyne's goroutine and nowhere else.
func (c *Chart) draw() {
	if c.live == nil {
		return
	}
	if c.stream != nil {
		c.stream.Snapshot()
	}
	c.releaseFollowed()
	if err := c.target.Render(c.live.Draw); err != nil {
		c.renderr = err
		return
	}
	c.renderr = nil
	if c.unnice() {
		// The axes were reframed, so the frame just drawn is the old framing.
		// Draw the new one rather than show a frame nobody asked for.
		if err := c.target.Render(c.live.Draw); err != nil {
			c.renderr = err
			return
		}
	}
	c.present()
}

// unnice takes the rounding off the axes the chart follows, and reports
// whether it had to.
//
// A linear axis nices by default: it rounds its domain outward to whole tick
// steps, which frames a static chart well and fights a moving one. A followed
// axis reframes itself every frame, so a niced one holds still while the data
// slides under it, then jumps a whole tick and holds still again — the oldest
// samples visibly leave the chart before the axis admits they are gone.
//
// The axis is rebuilt from its own description with that one flag cleared, so
// everything else about it — the kind, the base, the time zone, the padding —
// survives. An axis pinned to a domain is left alone, because a pinned domain
// is never niced anyway, and so is one carrying a formatter, which a
// description cannot hold and a rebuild would drop.
func (c *Chart) unnice() bool {
	if c.unniced || c.stream == nil || c.live == nil {
		return false
	}
	panels := c.live.Index().Panels()
	if len(panels) == 0 {
		return false
	}
	c.unniced = true

	changed := false
	if c.cfg.followX {
		changed = plainly(panels[0].X, c.plot.X) || changed
	}
	if c.cfg.followY {
		changed = plainly(panels[0].Y, c.plot.Y) || changed
	}
	if !changed {
		return false
	}
	// Rebuild resolves the plot again and paints nothing, so it needs no hold
	// on the surface.
	if err := c.live.Rebuild(); err != nil {
		c.renderr = err
		return false
	}
	return true
}

// plainly replaces s with the same scale minus its nicing, and reports whether
// it did.
func plainly(s scale.Scale, set func(scale.Scale) *figure.Plot) bool {
	d, ok := scale.Describe(s)
	if !ok || !d.Nice || d.Fixed || d.Formatted {
		return false
	}
	d.Nice = false
	plain, err := scale.FromDesc(d)
	if err != nil {
		return false
	}
	set(plain)
	return true
}

// follows reports whether any axis is tracking the data rather than standing
// where it was put.
func (c *Chart) follows() bool {
	return c.stream != nil && (c.cfg.followX || c.cfg.followY)
}

// steers reports whether a reader's drag or wheel is allowed to move the view.
//
// It is not, on a chart that follows its data unless [FollowPause] says so: a
// pan and the follow would fight over the same axis every frame, and the pan
// would lose — figure redraws from inside PanBy and Wheel, so a gesture frame
// shows the view the reader dragged to and the next frame snaps it back to the
// data. Ignoring the gesture is the honest version of what would happen
// anyway, without the flicker.
func (c *Chart) steers() bool { return !c.follows() || c.cfg.pause }

// tracksRows reports whether the chart should record which source row is
// behind each mark.
//
// [TrackRows] asks for it, and so does a drag that selects: a selection is
// [figure.Live.Select], which reads the rows out of the hit index, and an
// index that was not tracking them holds none — so a chart in
// [figure.DragSelects] that was not also told to track rows would drag out a
// rectangle and report nothing under it. Turning it on for the mode that needs
// it is not a default anybody would want overridden.
func (c *Chart) tracksRows() bool {
	return c.cfg.trackRows || c.cfg.drag == figure.DragSelects
}

// drags reports whether a press starts a gesture the chart will act on.
//
// It is [Chart.steers] for every drag that moves the view, and true regardless
// for [figure.DragSelects], which moves nothing: a chart following a stream
// still has rows a reader may want to mark out, and refusing the gesture there
// would be refusing it for a reason that does not apply. A drag that zooms to
// its band does move the view, and is refused with the pan.
func (c *Chart) drags() bool {
	return c.cfg.drag == figure.DragSelects || c.steers()
}

// releaseFollowed forgets what the followed axes were trained on, so that the
// frame about to be drawn establishes their domains from the rows the chart
// holds now rather than from every row it has ever held. See [Follow].
func (c *Chart) releaseFollowed() {
	if !c.follows() || (c.cfg.pause && c.steered) {
		return
	}
	for _, p := range c.live.Index().Panels() {
		if c.cfg.followX {
			release(p.X)
		}
		if c.cfg.followY {
			release(p.Y)
		}
	}
}

func release(s scale.Scale) {
	if z, ok := s.(scale.Zoomer); ok {
		z.Autoscale()
	}
}

// present shows the frame that was last drawn, without drawing one.
func (c *Chart) present() {
	if c.target != nil {
		c.target.Present()
	}
}

// hookEvents is where the tooltip learns what the pointer found. It goes
// through the plot's own event system rather than a second one, so a caller's
// handlers and the tooltip see the same hover.
func (c *Chart) hookEvents() {
	if c.hooked {
		return
	}
	c.hooked = true
	// A reader who has grabbed the chart has taken it off the follow: see
	// [Follow]. Both handlers are registered whether or not there is a
	// tooltip, because following is not a tooltip's business.
	c.plot.On(figure.Zoom, func(figure.Event) { c.steered = true; c.viewChanged() })
	c.plot.On(figure.Pan, func(figure.Event) { c.steered = true; c.viewChanged() })
	// A click on a legend row, when the chart was asked to wire one. It runs
	// inside the surface the release already holds — see [Chart.MouseUp] — so
	// the redraw Live.Toggle does needs no hold of its own, and it must not
	// take the chart's lock, which the same call already has.
	c.plot.On(figure.Click, func(ev figure.Event) {
		if !c.cfg.legendToggle || c.live == nil || ev.Hit.Kind != figure.LegendRow {
			return
		}
		if err := c.live.Toggle(ev.Hit.Layer); err != nil {
			c.renderr = err
		}
	})
	if !c.cfg.tooltip {
		return
	}
	c.plot.On(figure.Hover, func(ev figure.Event) { c.tip.show(ev) })
	c.plot.On(figure.Leave, func(figure.Event) { c.tip.hide() })
	c.plot.On(figure.Pan, func(figure.Event) { c.tip.hide() })
	c.plot.On(figure.Zoom, func(figure.Event) { c.tip.hide() })
}
