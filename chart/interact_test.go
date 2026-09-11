package chart_test

import (
	"math"
	"testing"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver/desktop"
	"github.com/timzifer/figure"
	"github.com/timzifer/figure/data"
	"github.com/timzifer/figure/geom"
	"github.com/timzifer/figure/ir"
	"github.com/timzifer/figure/palette"
	"github.com/timzifer/figure/scale"
	"github.com/timzifer/fyne_figure/chart"
)

func TestClickingALegendRowHidesTheLayer(t *testing.T) {
	c := chart.New(legendPlot(), chart.Interactive(true), chart.ThemeFont(false), chart.LegendToggle(true))
	shownAt(t, c)

	pos, hit := legendAt(t, c)
	if c.LayerHidden(hit.Layer) {
		t.Fatalf("layer %d (%q) was hidden before anything was clicked", hit.Layer, hit.Series)
	}

	click(c, pos)
	if !c.LayerHidden(hit.Layer) {
		t.Fatalf("clicking the legend row for %q left layer %d showing", hit.Series, hit.Layer)
	}
	click(c, pos)
	if c.LayerHidden(hit.Layer) {
		t.Fatalf("clicking it again left layer %d hidden", hit.Layer)
	}
	if err := c.Err(); err != nil {
		t.Errorf("drawing the toggled chart: %v", err)
	}
}

// The wiring is a switch and not a default: figure deliberately does not
// toggle a legend by itself, and a chart that was not asked to must not either.
func TestALegendDoesNotToggleUnlessAsked(t *testing.T) {
	c := chart.New(legendPlot(), chart.Interactive(true), chart.ThemeFont(false))
	shownAt(t, c)

	pos, hit := legendAt(t, c)
	click(c, pos)
	if c.LayerHidden(hit.Layer) {
		t.Error("a chart without LegendToggle hid a layer when its legend was clicked")
	}
}

func TestHidingALayerIsDrawnAndReported(t *testing.T) {
	c := chart.New(legendPlot(), chart.Interactive(true), chart.ThemeFont(false))
	shownAt(t, c)

	before := c.Target().Frames()
	if err := c.HideLayer(1, true); err != nil {
		t.Fatalf("hiding layer 1: %v", err)
	}
	if !c.LayerHidden(1) {
		t.Error("the layer was hidden and does not say so")
	}
	if c.Target().Frames() == before {
		t.Error("hiding a layer painted no frame")
	}
	if err := c.ShowAllLayers(); err != nil {
		t.Fatalf("showing every layer: %v", err)
	}
	if c.LayerHidden(1) {
		t.Error("ShowAllLayers left a layer hidden")
	}
}

// A drag in DragSelects reports the rows under the rectangle and does not move
// the chart, which is the whole difference from a pan.
func TestADragCanSelectInsteadOfPanning(t *testing.T) {
	c := chart.New(legendPlot(), chart.Interactive(true), chart.ThemeFont(false), chart.DragMode(figure.DragSelects))
	shownAt(t, c)

	var events []figure.Event
	c.Plot().On(figure.Select, func(ev figure.Event) { events = append(events, ev) })

	before := domains(c)
	band(c, fyne.NewPos(120, 80), fyne.NewPos(300, 220))

	if len(events) == 0 {
		t.Fatal("dragging a rectangle over the marks selected nothing")
	}
	for _, ev := range events {
		if len(ev.Rows) == 0 {
			t.Errorf("the selection of %q names no rows", ev.Hit.Series)
		}
	}
	if moved(before, domains(c)) {
		t.Error("a selection moved the view, which is what a pan does")
	}
	if err := c.Err(); err != nil {
		t.Errorf("drawing the selection: %v", err)
	}
}

// The band is the surface's to draw: figure paints nothing while one is being
// dragged out. So the frames have to come from here.
func TestARubberBandIsDrawnWhileItIsDraggedOut(t *testing.T) {
	c := chart.New(legendPlot(), chart.Interactive(true), chart.ThemeFont(false), chart.DragMode(figure.DragSelects))
	shownAt(t, c)

	press(c, 120, 80)
	before := c.Target().Frames()
	drag(c, 300, 220)
	if c.Target().Frames() == before {
		t.Error("dragging a selection out painted no frame, so nothing showed the band")
	}
	chart.PointerOf(c).DragEnd()
}

// A chart that follows a stream refuses a pan — it would fight the follow — and
// must not refuse a selection, which moves nothing.
func TestAFollowingChartCanStillBeSelectedOver(t *testing.T) {
	st := data.NewStream("t", "y").Window(200)
	for i := range 100 {
		v := float64(i) / 10
		if err := st.Append(v, math.Sin(v)); err != nil {
			t.Fatalf("appending to the stream: %v", err)
		}
	}
	p := figure.New(figure.Size(400, 250))
	p.X(scale.Linear())
	p.Add(geom.Line(st.Source(), geom.X("t"), geom.Y("y"), geom.Label("signal")))

	c := chart.New(p, chart.Interactive(true), chart.ThemeFont(false), chart.DragMode(figure.DragSelects))
	c.Stream(st)
	shownAt(t, c)

	var got int
	c.Plot().On(figure.Select, func(figure.Event) { got++ })
	band(c, fyne.NewPos(120, 80), fyne.NewPos(320, 220))
	if got == 0 {
		t.Error("a chart following its data refused a selection, which moves nothing")
	}
}

func TestAnOverlayIsInstalledAndOnlyWhenThereIsOne(t *testing.T) {
	plain := chart.New(legendPlot(), chart.Interactive(true), chart.ThemeFont(false))
	shownAt(t, plain)
	// Nothing installed is not a detail: figure redraws on every hover while
	// an overlay is installed, so a chart carrying an empty one would pay a
	// frame per pointer move.
	if plain.Live().CurrentOverlay() != nil {
		t.Error("a chart with no overlay installed one anyway")
	}

	cross := &figure.Crosshair{}
	c := chart.New(legendPlot(), chart.Interactive(true), chart.ThemeFont(false), chart.Overlay(cross))
	shownAt(t, c)
	if c.CurrentOverlay() != figure.Overlay(cross) {
		t.Error("the overlay the chart was built with is not the one it reports")
	}
	if c.Live().CurrentOverlay() == nil {
		t.Fatal("the overlay the chart was built with was never installed")
	}

	// It draws: a crosshair over a point is two rules that were not there.
	cross.At, cross.Show = ir.Point{X: 200, Y: 150}, true
	before := c.Target().Frames()
	c.Redraw()
	if c.Target().Frames() == before {
		t.Error("showing the crosshair painted no frame")
	}
}

func TestAnOverlayCanBeInstalledAfterTheChartIsShown(t *testing.T) {
	c := chart.New(legendPlot(), chart.Interactive(true), chart.ThemeFont(false))
	shownAt(t, c)

	cross := &figure.Crosshair{At: ir.Point{X: 200, Y: 150}, Show: true}
	before := c.Target().Frames()
	c.Overlay(cross)
	if c.Live().CurrentOverlay() == nil {
		t.Fatal("installing an overlay on a shown chart did not install it")
	}
	if c.Target().Frames() == before {
		t.Error("installing an overlay painted no frame")
	}

	c.Overlay(nil)
	if c.Live().CurrentOverlay() != nil {
		t.Error("removing the overlay left one installed")
	}
}

func TestTheViewOfOneChartCanBePutIntoAnother(t *testing.T) {
	left := chart.New(legendPlot(), chart.Interactive(true), chart.ThemeFont(false))
	right := chart.New(legendPlot(), chart.Interactive(true), chart.ThemeFont(false))
	shownAt(t, left)
	shownAt(t, right)

	var told int
	left.OnViewChange(func(v figure.View) {
		told++
		if err := right.SetView(v); err != nil {
			t.Errorf("putting the view into the other chart: %v", err)
		}
	})
	// And the other way, which is the arrangement that would loop if SetView
	// reported the view it was given.
	right.OnViewChange(func(v figure.View) {
		if err := left.SetView(v); err != nil {
			t.Errorf("putting the view back: %v", err)
		}
	})

	before := domains(right)
	scroll(left, fyne.NewPos(250, 150), 3)

	if told == 0 {
		t.Fatal("zooming the chart told nobody its view had changed")
	}
	if !moved(before, domains(right)) {
		t.Error("the linked chart did not follow the one that was zoomed")
	}
}

// A view that was put there is not a view the reader moved to. If SetView
// reported one, two linked charts would tell each other for ever.
func TestSetViewReportsNothing(t *testing.T) {
	c := chart.New(legendPlot(), chart.Interactive(true), chart.ThemeFont(false))
	shownAt(t, c)

	v := c.View()
	var told int
	c.OnViewChange(func(figure.View) { told++ })
	if err := c.SetView(v); err != nil {
		t.Fatalf("putting the view back: %v", err)
	}
	if told != 0 {
		t.Errorf("SetView reported the view %d times", told)
	}
}

// The double click that puts the view back fires no event of figure's own, so
// the chart has to say so itself or a linked chart would stay zoomed.
func TestGoingBackToTheWholePictureIsReported(t *testing.T) {
	c := chart.New(legendPlot(), chart.Interactive(true), chart.ThemeFont(false))
	shownAt(t, c)

	scroll(c, fyne.NewPos(250, 150), 3)
	var told int
	c.OnViewChange(func(figure.View) { told++ })
	chart.PointerOf(c).DoubleTapped(&fyne.PointEvent{Position: fyne.NewPos(250, 150)})
	if told == 0 {
		t.Error("a double click put the view back and told nobody")
	}
}

func TestATransitionIsDrivenToItsEnd(t *testing.T) {
	before := figure.NewTable().
		String("lang", []string{"go", "rust", "perl"}).
		Float64("slot", []float64{0, 1, 2}).
		Float64("share", []float64{40, 25, 18})
	after := figure.NewTable().
		String("lang", []string{"go", "rust", "zig"}).
		Float64("slot", []float64{0, 1, 2}).
		Float64("share", []float64{30, 45, 22})

	tw, err := data.NewTween(before, after, "lang",
		data.EnterFrom("share", 0), data.ExitTo("share", 0), data.Hold("slot"))
	if err != nil {
		t.Fatalf("building the tween: %v", err)
	}

	p := figure.New(figure.Size(400, 250))
	p.X(scale.Linear(scale.Domain(-0.6, 2.6)))
	p.Y(scale.Linear(scale.Domain(0, 50)))
	p.Add(geom.Bar(tw.Source(), geom.X("slot"), geom.Y("share"),
		geom.KeyBy("lang"), geom.Color(palette.Blue), geom.BarWidth(0.6)))

	c := chart.New(p, chart.Interactive(true), chart.ThemeFont(false))
	shownAt(t, c)

	tr, err := c.Transition(tw)
	if err != nil {
		t.Fatalf("building the transition: %v", err)
	}
	if tr == nil {
		t.Fatal("a chart that has been laid out has no transition to give")
	}

	frames := c.Target().Frames()
	// A transition being played belongs to the goroutine playing it, so this
	// is how a test learns it is over: reading tr.Done() from here would be
	// reading it while the driver writes it.
	over := make(chan struct{})
	c.Play(tr.Over(80*time.Millisecond), func() { close(over) })

	select {
	case <-over:
	case <-time.After(5 * time.Second):
		t.Fatal("the transition never reached its end")
	}
	if c.Target().Frames() <= frames {
		t.Error("a transition ran and painted nothing")
	}
	if err := c.Err(); err != nil {
		t.Errorf("drawing the transition: %v", err)
	}
}

func TestPlayingATransitionOnAChartWithoutOneIsHarmless(t *testing.T) {
	c := chart.New(legendPlot(), chart.Interactive(true), chart.ThemeFont(false))
	// Before any layout there is no Live, so there is nothing to build one on.
	tr, err := c.Transition()
	if tr != nil || err != nil {
		t.Errorf("a chart that has not been laid out gave %v, %v", tr, err)
	}
	c.Play(nil, nil)()
}

// click presses and releases at one position, which is what figure's own
// click slop turns into a click rather than a drag.
func click(c *chart.Chart, pos fyne.Position) {
	ev := &desktop.MouseEvent{PointEvent: fyne.PointEvent{Position: pos}, Button: desktop.MouseButtonPrimary}
	chart.PointerOf(c).MouseDown(ev)
	chart.PointerOf(c).MouseUp(ev)
}

// band presses at one corner, moves to the other and releases — the three
// events Fyne splits a drag into.
func band(c *chart.Chart, from, to fyne.Position) {
	press(c, from.X, from.Y)
	drag(c, to.X, to.Y)
	chart.PointerOf(c).DragEnd()
}

func scroll(c *chart.Chart, at fyne.Position, notches float32) {
	chart.PointerOf(c).Scrolled(&fyne.ScrollEvent{
		PointEvent: fyne.PointEvent{Position: at},
		Scrolled:   fyne.NewDelta(0, notches),
	})
	// A wheel is paced, so the frame it asked for may still be waiting.
	chart.PointerOf(c).MouseOut()
}

// domains is where every axis of every panel currently reaches. A
// [figure.View] carries the same numbers and does not hand them out — it is a
// value to put back, not one to read — so a test that wants to say "the view
// moved" reads them off the scales.
func domains(c *chart.Chart) []float64 {
	live := c.Live()
	if live == nil {
		return nil
	}
	var out []float64
	for _, p := range live.Index().Panels() {
		for _, s := range []scale.Scale{p.X, p.Y} {
			if s == nil {
				continue
			}
			lo, hi := s.Domain()
			out = append(out, lo, hi)
		}
	}
	return out
}

// moved reports whether the axes are somewhere else than they were.
func moved(a, b []float64) bool {
	if len(a) != len(b) {
		return true
	}
	for i := range a {
		if a[i] != b[i] {
			return true
		}
	}
	return false
}
