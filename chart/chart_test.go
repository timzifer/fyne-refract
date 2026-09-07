package chart_test

import (
	"image"
	"math"
	"testing"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/test"
	"github.com/timzifer/fyne-refract/chart"
	"github.com/timzifer/refract"
	"github.com/timzifer/refract/data"
	"github.com/timzifer/refract/geom"
	"github.com/timzifer/refract/scale"
)

// What is asserted here is what refract reports, not what the chart looks
// like: the widget's job is to turn Fyne's events into refract's, and a test
// that scraped pixels for it would be testing the rasterizer instead.

func TestAChartIsOpenedAtTheSizeItIsLaidOutAt(t *testing.T) {
	c, _ := shown(t, fyne.NewSize(500, 300))

	if c.Live() == nil {
		t.Fatal("laying the widget out did not open a chart")
	}
	if err := c.Err(); err != nil {
		t.Fatalf("the first frame failed: %v", err)
	}
	if w, h := c.Live().Size(); w != 500 || h != 300 {
		t.Errorf("the chart is %dx%d, want the size it was laid out at, 500x300", w, h)
	}
}

func TestAChartFollowsAResize(t *testing.T) {
	c, _ := shown(t, fyne.NewSize(500, 300))
	c.Resize(fyne.NewSize(640, 400))

	if w, h := c.Live().Size(); w != 640 || h != 400 {
		t.Errorf("after a resize the chart is %dx%d, want 640x400", w, h)
	}
	if err := c.Err(); err != nil {
		t.Errorf("the frame after a resize failed: %v", err)
	}
}

func TestAWheelZoomsAboutThePointer(t *testing.T) {
	c, win := shown(t, fyne.NewSize(500, 300))
	x := c.Plot()

	before := domainOf(t, c)
	test.Scroll(win.Canvas(), fyne.NewPos(250, 150), 0, 4)
	after := domainOf(t, c)

	if after == before {
		t.Fatalf("the wheel changed nothing; the x domain is still %v", before)
	}
	if width(after) >= width(before) {
		t.Errorf("scrolling up widened the domain from %v to %v, want a zoom in", before, after)
	}
	_ = x
}

func TestADoubleClickPutsTheViewBack(t *testing.T) {
	c, win := shown(t, fyne.NewSize(500, 300))

	before := domainOf(t, c)
	test.Scroll(win.Canvas(), fyne.NewPos(250, 150), 0, 4)
	if domainOf(t, c) == before {
		t.Fatal("the wheel changed nothing, so there is no zoom to undo")
	}

	c.DoubleTapped(&fyne.PointEvent{Position: fyne.NewPos(250, 150)})
	if got := domainOf(t, c); got != before {
		t.Errorf("after a double click the domain is %v, want the original %v", got, before)
	}
}

func TestADragPansAndDoesNotClick(t *testing.T) {
	c, win := shown(t, fyne.NewSize(500, 300))

	var clicks, pans int
	c.Plot().On(refract.Click, func(refract.Event) { clicks++ })
	c.Plot().On(refract.Pan, func(refract.Event) { pans++ })

	// Fyne's own test.Drag reports a drag with no press behind it and never
	// moves the pointer, so the press, the move and the release are sent the
	// way a desktop driver sends them.
	before := domainOf(t, c)
	c.MouseDown(&desktop.MouseEvent{
		PointEvent: fyne.PointEvent{Position: fyne.NewPos(250, 150)},
		Button:     desktop.MouseButtonPrimary,
	})
	c.Dragged(&fyne.DragEvent{
		PointEvent: fyne.PointEvent{Position: fyne.NewPos(310, 150)},
		Dragged:    fyne.NewDelta(60, 0),
	})
	c.DragEnd()
	_ = win

	if pans == 0 {
		t.Error("dragging the chart fired no pan")
	}
	if clicks != 0 {
		t.Errorf("dragging the chart fired %d clicks, want none", clicks)
	}
	if got := domainOf(t, c); got == before {
		t.Errorf("the domain is still %v after a pan", before)
	}
}

func TestHoveringReportsWhatIsUnderThePointer(t *testing.T) {
	c, win := shown(t, fyne.NewSize(500, 300))

	var found bool
	c.Plot().On(refract.Hover, func(ev refract.Event) { found = found || ev.Found })

	// Sweep the plot area: a line drawn across it is under the pointer
	// somewhere along the way, wherever the margins happen to fall.
	for x := float32(60); x < 460; x += 10 {
		for y := float32(40); y < 260; y += 10 {
			test.MoveMouse(win.Canvas(), fyne.NewPos(x, y))
			if found {
				return
			}
		}
	}
	t.Error("no position over the chart found a mark")
}

func TestAnUnchangedChartIsNotDrawnTwice(t *testing.T) {
	c, _ := shown(t, fyne.NewSize(500, 300))

	c.Refresh()
	first := frames(t, c)
	c.Refresh()
	if got := frames(t, c); got != first {
		t.Errorf("refreshing an unchanged chart painted %d frames, want the %d already painted", got, first)
	}
}

func TestAStreamIsFrozenBeforeEachFrame(t *testing.T) {
	st := data.NewStream("t", "y").Window(200)
	for i := range 50 {
		if err := st.Append(float64(i), math.Sin(float64(i)/5)); err != nil {
			t.Fatalf("appending: %v", err)
		}
	}

	p := refract.New(refract.Size(400, 250))
	p.Add(geom.Line(st.Source(), geom.X("t"), geom.Y("y")))
	c := chart.New(p, chart.ThemeFont(false))
	c.Stream(st)

	win := test.NewTempWindow(t, c)
	win.Resize(fyne.NewSize(500, 300))
	c.Resize(fyne.NewSize(500, 300))
	if err := c.Err(); err != nil {
		t.Fatalf("the first frame failed: %v", err)
	}
	before := frames(t, c)

	for i := 50; i < 120; i++ {
		if err := st.Append(float64(i), math.Sin(float64(i)/5)); err != nil {
			t.Fatalf("appending: %v", err)
		}
	}
	c.Refresh()

	if got := frames(t, c); got == before {
		t.Error("appending to the stream and redrawing painted no frame")
	}
}

func TestAnimateStopsWhenItIsTold(t *testing.T) {
	c, _ := shown(t, fyne.NewSize(400, 250))
	stop := c.Animate(10 * time.Millisecond)
	stop()
	stop() // stopping twice is not a panic
}

// shown builds a chart in a test window and lays it out.
func shown(t *testing.T, size fyne.Size, opts ...chart.Option) (*chart.Chart, fyne.Window) {
	t.Helper()
	c := chart.New(plot(), append([]chart.Option{chart.ThemeFont(false)}, opts...)...)
	win := test.NewTempWindow(t, c)
	win.Resize(size)
	c.Resize(size)
	if err := c.Err(); err != nil {
		t.Fatalf("laying the chart out: %v", err)
	}
	return c, win
}

func domainOf(t *testing.T, c *chart.Chart) [2]float64 {
	t.Helper()
	panels := c.Live().Index().Panels()
	if len(panels) == 0 {
		t.Fatal("the chart reported no panels")
	}
	lo, hi := panels[0].X.Domain()
	return [2]float64{float64(lo), float64(hi)}
}

func width(d [2]float64) float64 { return math.Abs(d[1] - d[0]) }

func frames(t *testing.T, c *chart.Chart) uint64 {
	t.Helper()
	return c.Target().Frames()
}

func benchPlot() *refract.Plot { return plot() }

func plot() *refract.Plot {
	const n = 200
	x := make([]float64, n)
	y := make([]float64, n)
	for i := range n {
		v := float64(i) / 20
		x[i], y[i] = v, math.Sin(v)
	}
	src := refract.Float64Columns(map[string][]float64{"t": x, "y": y})

	p := refract.New(refract.Size(400, 250), refract.Title("Signal"))
	p.X(scale.Linear())
	p.Add(geom.Line(src, geom.X("t"), geom.Y("y"), geom.Label("signal")))
	return p
}

func TestTheChartRasterizesAtTheRatioThePainterAsksFor(t *testing.T) {
	c, _ := shown(t, fyne.NewSize(500, 300))

	if _, _, dpr := c.Target().Size(); dpr != 1 {
		t.Fatalf("the chart opened at dpr %v, want 1", dpr)
	}

	// A window dragged onto a display with twice the pixels: the painter asks
	// for a frame twice the size, and the chart rasterizes the next one at it.
	raster := c.Target().Object().(*canvas.Raster)
	raster.Generator(1000, 600)
	c.Refresh()

	if _, _, dpr := c.Target().Size(); dpr != 2 {
		t.Errorf("the chart rasterizes at dpr %v, want 2", dpr)
	}
	if b := c.Target().Image().Bounds(); b.Dx() != 1000 || b.Dy() != 600 {
		t.Errorf("the buffer is %v, want 1000x600 device pixels", b)
	}
	if w, h := c.Live().Size(); w != 500 || h != 300 {
		t.Errorf("the chart is laid out at %dx%d, want the logical 500x300", w, h)
	}
}

func TestAChartThatWasNeverLaidOutDoesNothing(t *testing.T) {
	test.NewTempApp(t)
	c := chart.New(plot(), chart.ThemeFont(false))

	if c.Live() != nil {
		t.Error("a chart that was never laid out has a live chart")
	}
	if err := c.Rebuild(); err != nil {
		t.Errorf("rebuilding: %v", err)
	}
	if err := c.Autoscale(); err != nil {
		t.Errorf("autoscaling: %v", err)
	}
	c.MouseOut()
	c.DoubleTapped(&fyne.PointEvent{})
	c.Scrolled(&fyne.ScrollEvent{Scrolled: fyne.NewDelta(0, 4)})
	c.DragEnd()

	if err := c.Close(); err != nil {
		t.Errorf("closing: %v", err)
	}
}

func TestClosingTwiceIsNotAnError(t *testing.T) {
	c, _ := shown(t, fyne.NewSize(400, 250))
	if err := c.Close(); err != nil {
		t.Fatalf("closing: %v", err)
	}
	if err := c.Close(); err != nil {
		t.Errorf("closing a closed chart: %v", err)
	}
}

func TestTheChartReachesFynesPainter(t *testing.T) {
	c, win := shown(t, fyne.NewSize(500, 300))

	// Fyne's software painter draws the whole canvas, raster included. If the
	// chart never reached it the capture would be one flat colour.
	img := win.Canvas().Capture()
	if img == nil {
		t.Fatal("the canvas captured nothing")
	}
	if n := distinctColours(img); n < 3 {
		t.Errorf("the painted canvas has %d distinct colours, want a chart's worth", n)
	}
	_ = c
}

// distinctColours counts up to a handful of different colours in an image,
// which is enough to tell a drawing from a blank.
func distinctColours(img image.Image) int {
	seen := map[[4]uint32]struct{}{}
	b := img.Bounds()
	for y := b.Min.Y; y < b.Max.Y && len(seen) < 8; y++ {
		for x := b.Min.X; x < b.Max.X && len(seen) < 8; x++ {
			r, g, bl, a := img.At(x, y).RGBA()
			seen[[4]uint32{r, g, bl, a}] = struct{}{}
		}
	}
	return len(seen)
}

func TestAChartThatCouldNotOpenTriesAgain(t *testing.T) {
	test.NewTempApp(t)

	// A plot with no layers and no scales cannot be drawn, and says so.
	p := refract.New(refract.Size(400, 250))
	c := chart.New(p, chart.ThemeFont(false))
	win := test.NewTempWindow(t, c)
	win.Resize(fyne.NewSize(500, 300))
	c.Resize(fyne.NewSize(500, 300))

	if c.Err() == nil {
		t.Fatal("a plot with no layers opened a chart")
	}
	if c.Live() != nil {
		t.Fatal("a chart that could not open has a live chart")
	}

	// Giving it something to draw and laying it out again is all it takes:
	// the target is reusable, whether or not the failed attempt closed it.
	p.Add(geom.Line(source(), geom.X("t"), geom.Y("y")))
	c.Resize(fyne.NewSize(501, 301))

	if err := c.Err(); err != nil {
		t.Fatalf("the second attempt failed too: %v", err)
	}
	if c.Live() == nil {
		t.Fatal("the chart did not open on the second attempt")
	}
	if c.Target().Frames() == 0 {
		t.Error("the chart opened but painted nothing")
	}
}

func source() refract.Source {
	const n = 100
	x := make([]float64, n)
	y := make([]float64, n)
	for i := range n {
		v := float64(i) / 10
		x[i], y[i] = v, math.Cos(v)
	}
	return refract.Float64Columns(map[string][]float64{"t": x, "y": y})
}
