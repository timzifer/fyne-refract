package orbit

import (
	"image"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/widget"
	"github.com/timzifer/figure/interact"
	"github.com/timzifer/figure/three"
	fynefigure "github.com/timzifer/fyne-figure"
	"github.com/timzifer/fyne-figure/internal/look"
)

// Which views a gesture turns: one view by its index, or one of these.
const (
	everyView = -1
	noView    = -2
)

// Chart is a three-dimensional figure plot as a Fyne widget.
//
// It is drawn by [fynefigure.Target] — the same rasterizer, the same pixels,
// as a flat chart and as the PNG the same plot would write — and turned by
// [three.Live], which it tells where the cameras are and asks for frames.
//
// A reader turns it once it is [Interactive], and not before. Until then it
// takes no pointer events at all.
//
// The chart is opened at the first layout, because a plot needs a size and a
// widget has none until it is laid out. Everything before that — the scene,
// its layers, its views — belongs on the plot.
type Chart struct {
	widget.BaseWidget

	plot *three.Plot
	cfg  config

	target *fynefigure.Target
	live   *three.Live

	// w and h are the logical size the chart was last laid out at, and dpr the
	// device pixel ratio it was last rasterized at.
	w, h int
	dpr  float64

	// lock serialises everything the chart does, for the reason it does in
	// package chart: events arrive on Fyne's goroutine and the pacing timer on
	// another, and holding this is correct whichever driver is running.
	lock sync.Mutex

	// painterPx is the pixel geometry the painter last asked for when it
	// disagreed with what had been rasterized. It is written from the painter
	// goroutine, which is why it has a lock of its own.
	mu        sync.Mutex
	painterPx image.Point

	// The gesture. dragging says a drag has started and turning which views it
	// turns; dAz and dEl are the part of it no frame has shown yet. wheel and
	// wheelView are the same for the wheel.
	dragging  bool
	turning   int
	dAz, dEl  float64
	wheel     float64
	wheelView int

	// Pacing: see pace.go.
	lastFrame time.Time
	frameCost time.Duration
	timer     *time.Timer
	pending   bool

	// touched are the views whose cameras a reader moved in the frame being
	// drawn, for OnCamera.
	touched []int

	// cameras carries what the reader turned each view to across a reopen,
	// which is what a change of theme is.
	cameras []three.Camera

	// themed is what the chart was last built for; see look.State.
	themed look.State

	// ptr is the layer that takes the pointer. It is always in the widget's
	// tree and hidden unless the chart is [Interactive]; see input.go.
	ptr *pointer

	onCamera func(view int, cam three.Camera)
	onHover  func(h interact.Hit, found bool)
	hovering bool

	// The selection: what the reader picked, what rings it in every view, and
	// who is told. See select.go.
	sel      fynefigure.Selection
	mk       *marks
	onSelect func(fynefigure.Selection)

	// down is where a press landed, and pressed that there is one, so that a
	// release can tell a click from the end of a drag.
	down    fyne.Position
	pressed bool

	// The level of detail: whether frames are drawn below the screen's
	// resolution, whether a gesture has been reported begun, who is told, and
	// what ends a wheel. See detail.go.
	coarse     bool
	gesturing  bool
	onGesture  func(active bool)
	wheelTimer *time.Timer

	renderr error
}

// A chart is a widget and nothing else. The pointer interfaces are its
// pointer layer's, which is only there to be found once the chart is
// [Interactive] — see input.go.
var _ fyne.Widget = (*Chart)(nil)

// New returns a widget showing p.
//
// Nothing is rasterized until the widget is laid out, so a chart built and
// never shown has taken no memory beyond the plot itself.
func New(p *three.Plot, opts ...Option) *Chart {
	c := &Chart{plot: p, cfg: defaults(), dpr: 1, turning: noView, wheelView: noView}
	for _, o := range opts {
		o(&c.cfg)
	}
	c.ptr = newPointer(c, c.cfg.interactive)
	c.mk = &marks{ring: c.cfg.ring}
	c.ExtendBaseWidget(c)
	return c
}

// Plot returns the plot the chart shows.
func (c *Chart) Plot() *three.Plot { return c.plot }

// Live returns the scene being drawn, or nil before the first layout. A caller
// that drives it directly should call [Chart.Refresh] afterwards.
func (c *Chart) Live() *three.Live { return c.live }

// Target returns what the chart is drawn into, or nil before the first
// layout. An export of exactly what is on screen reads
// [fynefigure.Target.Image].
func (c *Chart) Target() *fynefigure.Target { return c.target }

// Err reports what went wrong in the last frame, if anything. A widget has
// nowhere to return an error to, so a failed frame is kept here and the widget
// shows the frame before it.
func (c *Chart) Err() error { return c.renderr }

// Refresh redraws the chart. It is what to call from a button, a menu or any
// other place that already runs on Fyne's goroutine.
func (c *Chart) Refresh() { c.BaseWidget.Refresh() }

// Redraw asks for a frame from anywhere. The frame is drawn on Fyne's
// goroutine in its turn.
func (c *Chart) Redraw() { fyne.Do(c.locked(c.draw)) }

// locked wraps an operation so that it holds the chart while it runs.
func (c *Chart) locked(fn func()) func() {
	return func() {
		c.lock.Lock()
		defer c.lock.Unlock()
		fn()
	}
}

// ViewCount reports how many views the plot has.
func (c *Chart) ViewCount() int {
	c.lock.Lock()
	defer c.lock.Unlock()
	if c.live != nil {
		return c.live.ViewCount()
	}
	return len(c.plot.Views())
}

// Camera reports where view i is looking from: the camera a reader turned it
// to, or before the first layout the one its author chose. It is the zero
// camera for an index that is not a view.
func (c *Chart) Camera(i int) three.Camera {
	c.lock.Lock()
	defer c.lock.Unlock()
	if c.live != nil {
		return c.live.CameraOf(i)
	}
	if views := c.plot.Views(); i >= 0 && i < len(views) {
		return views[i].Camera
	}
	return three.Camera{}
}

// SetCamera points view i at cam and redraws. It reports nothing to
// [Chart.OnCamera]: a caller moving a camera already knows, and a link between
// two charts would otherwise echo for ever.
func (c *Chart) SetCamera(i int, cam three.Camera) error {
	c.lock.Lock()
	defer c.lock.Unlock()
	if c.live == nil {
		return nil
	}
	c.live.SetCamera(i, cam)
	c.draw()
	return c.renderr
}

// Home returns every view to the camera its author chose, and redraws. It is
// what a double click does.
func (c *Chart) Home() error {
	c.lock.Lock()
	defer c.lock.Unlock()
	c.home()
	return c.renderr
}

// home is Home with the lock held. It reports every view, because every view
// may have moved.
func (c *Chart) home() {
	if c.live == nil {
		return
	}
	c.stopGesture()
	c.live.Home()
	for i := range c.live.ViewCount() {
		c.touch(i)
	}
	c.draw()
	c.report()
}

// OnCamera registers what to call when a reader moves a camera — a drag, a
// wheel, a double click — with the view it moved and where it now looks from.
// It is called once per view per frame drawn. See the package documentation
// for the link it is for.
func (c *Chart) OnCamera(fn func(view int, cam three.Camera)) {
	c.lock.Lock()
	defer c.lock.Unlock()
	c.onCamera = fn
}

// OnHover registers what to call when the pointer finds a mark, and once more
// with found false when it stops finding one. The hit names the view as its
// Panel and the layer and series as it does in a flat chart; its X and Y are
// meaningless in a projected scene, and its Row is -1 unless [TrackRows] asked
// for it.
func (c *Chart) OnHover(fn func(h interact.Hit, found bool)) {
	c.lock.Lock()
	defer c.lock.Unlock()
	c.onHover = fn
}

// Close releases the chart's pixels and stops anything pending.
//
// A widget removed from its tree is closed for you when Fyne destroys its
// renderer; this is for a caller who wants it gone sooner.
func (c *Chart) Close() error {
	c.lock.Lock()
	defer c.lock.Unlock()
	return c.close()
}

func (c *Chart) close() error {
	if c.timer != nil {
		c.timer.Stop()
		c.timer = nil
	}
	c.stopGesture()
	c.gestureEnds()
	var err error
	if c.live != nil {
		err = c.live.Close()
		c.live = nil
	}
	if c.target != nil {
		if cerr := c.target.Close(); err == nil {
			err = cerr
		}
		c.target = nil
	}
	return err
}

// CreateRenderer is called by Fyne. It is not part of the API.
func (c *Chart) CreateRenderer() fyne.WidgetRenderer {
	c.lock.Lock()
	defer c.lock.Unlock()
	c.ExtendBaseWidget(c)
	c.ensureTarget()
	return &renderer{c: c, objects: []fyne.CanvasObject{c.target.Object(), c.ptr}}
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
		for i, cam := range c.cameras {
			c.live.SetCamera(i, cam)
		}
		c.cameras = c.cameras[:0]
		// The plot's own size and ratio, which is what Live opened at; the
		// ones the widget wants are applied below.
		c.w, c.h = c.plot.Size()
		c.dpr = 0
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

// ensureTarget builds the render target.
func (c *Chart) ensureTarget() {
	if c.target != nil {
		return
	}
	c.target = fynefigure.New()
	c.target.OnGeometry(c.painterGeometry)
}

// painterGeometry is called from Fyne's painter when it is about to draw the
// chart at a pixel size the chart was not rasterized at — a window moved to a
// display with a different device pixel ratio. It records the size and nothing
// else; the refresh that follows is where [Chart.checkScale] reads it.
func (c *Chart) painterGeometry(widthPx, heightPx int) {
	c.mu.Lock()
	defer c.mu.Unlock()
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
	// The screen's ratio is what is remembered; a coarse chart rasterizes at
	// its part of it. See detail.go.
	was := c.dpr
	c.dpr = dpr
	if err := c.target.Render(func() error { return c.live.Rescale(c.rasterScale()) }); err != nil {
		c.dpr = was
		c.renderr = err
	}
}

// scaleFactor is the device pixel ratio the chart should rasterize at: the
// painter's, once it has painted, and the canvas's until then.
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

// draw paints a frame and shows it. It runs on Fyne's goroutine and nowhere
// else.
func (c *Chart) draw() {
	if c.live == nil {
		return
	}
	if err := c.target.Render(c.live.Draw); err != nil {
		c.renderr = err
		return
	}
	c.renderr = nil
	c.target.Present()
}
