package orbit_test

import (
	"slices"
	"sync"
	"testing"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"github.com/timzifer/figure/three"
	"github.com/timzifer/fyne_figure/orbit"
)

// A coarse chart rasterizes a fraction of the pixels and a sharp one all of
// them. What is asserted is the size of the frame the rasterizer produced,
// which is the whole of what the option changes.
func TestACoarseChartDrawsFewerPixelsUntilItIsSharpenedAgain(t *testing.T) {
	c, _ := shown(t, fyne.NewSize(500, 300), plot(), orbit.Detail(0.5))
	full := c.Target().Image().Bounds().Dx()

	c.SetCoarse(true)
	if !c.Coarse() {
		t.Fatal("SetCoarse(true) left the chart sharp")
	}
	if err := c.SetCamera(0, three.Orbit(c.Camera(0), 0.3, 0)); err != nil {
		t.Fatal(err)
	}
	if got := c.Target().Image().Bounds().Dx(); got*4 > full*3 {
		t.Errorf("a coarse frame is %d pixels wide, want about half of %d", got, full)
	}

	c.SetCoarse(false)
	if got := c.Target().Image().Bounds().Dx(); got != full {
		t.Errorf("after sharpening the frame is %d pixels wide, want %d again", got, full)
	}
}

// A chart that was not given a Detail below one has nothing to trade, so being
// told to go coarse is not an error and changes nothing.
func TestAChartWithoutDetailStaysSharp(t *testing.T) {
	c, _ := shown(t, fyne.NewSize(500, 300), plot())
	full := c.Target().Image().Bounds().Dx()

	c.SetCoarse(true)
	if c.Coarse() {
		t.Error("a chart with no Detail reports itself coarse")
	}
	if err := c.SetCamera(0, three.Orbit(c.Camera(0), 0.3, 0)); err != nil {
		t.Fatal(err)
	}
	if got := c.Target().Image().Bounds().Dx(); got != full {
		t.Errorf("the frame is %d pixels wide, want %d", got, full)
	}
}

// A drag is one gesture however many events it takes, and it ends when it is
// let go.
func TestOnGestureReportsADragOnceEachWay(t *testing.T) {
	c, _ := shown(t, fyne.NewSize(500, 300), plot())
	var got []bool
	c.OnGesture(func(active bool) { got = append(got, active) })

	from := fyne.NewPos(250, 150)
	for range 3 {
		step := fyne.NewDelta(10, 0)
		from = from.Add(step)
		orbit.PointerOf(c).Dragged(&fyne.DragEvent{PointEvent: fyne.PointEvent{Position: from}, Dragged: step})
	}
	if !slices.Equal(got, []bool{true}) {
		t.Fatalf("during a drag of three steps OnGesture said %v, want [true]", got)
	}
	orbit.PointerOf(c).DragEnd()
	if !slices.Equal(got, []bool{true, false}) {
		t.Errorf("after the drag OnGesture said %v, want [true false]", got)
	}
}

// A wheel has no end to report, so it is over once it has been still.
func TestOnGestureEndsAWheelOnceItIsStill(t *testing.T) {
	c, win := shown(t, fyne.NewSize(500, 300), plot())
	var mu sync.Mutex
	var got []bool
	c.OnGesture(func(active bool) {
		mu.Lock()
		got = append(got, active)
		mu.Unlock()
	})
	said := func() []bool {
		mu.Lock()
		defer mu.Unlock()
		return slices.Clone(got)
	}

	test.Scroll(win.Canvas(), fyne.NewPos(250, 150), 0, 4)
	test.Scroll(win.Canvas(), fyne.NewPos(250, 150), 0, 4)
	if s := said(); !slices.Equal(s, []bool{true}) {
		t.Fatalf("two notches of the wheel made OnGesture say %v, want [true]", s)
	}

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) && len(said()) < 2 {
		time.Sleep(10 * time.Millisecond)
	}
	if s := said(); !slices.Equal(s, []bool{true, false}) {
		t.Errorf("once the wheel was still OnGesture had said %v, want [true false]", s)
	}
}

// The link the two exist for: the chart being turned tells the other to go
// coarse, and to sharpen when it lets go.
func TestTheChartBeingTurnedCanCoarsenAnother(t *testing.T) {
	a, _ := shown(t, fyne.NewSize(500, 300), plot())
	b, _ := shown(t, fyne.NewSize(500, 300), plot(), orbit.Detail(0.5))
	a.OnGesture(func(active bool) { b.SetCoarse(active) })

	orbit.PointerOf(a).Dragged(&fyne.DragEvent{
		PointEvent: fyne.PointEvent{Position: fyne.NewPos(260, 150)}, Dragged: fyne.NewDelta(10, 0),
	})
	if !b.Coarse() {
		t.Error("while the first chart was turned the second stayed sharp")
	}
	orbit.PointerOf(a).DragEnd()
	if b.Coarse() {
		t.Error("after the turn the second chart was left coarse")
	}
}
