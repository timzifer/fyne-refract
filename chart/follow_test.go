package chart_test

import (
	"math"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/test"
	"github.com/timzifer/figure"
	"github.com/timzifer/figure/data"
	"github.com/timzifer/figure/geom"
	"github.com/timzifer/figure/scale"
	"github.com/timzifer/fyne_figure/chart"
)

// A scale is trained, and training accumulates: a domain grows and never
// shrinks. That is right for a table and wrong for a sliding window, whose
// oldest row leaves on every frame — so the axis of a followed chart has to be
// established again from the rows that are there.

func TestAFollowedAxisLeavesTheOldestDataBehind(t *testing.T) {
	c, st := streaming(t, chart.ThemeFont(false))

	first := domainOf(t, c)
	if first[0] != 0 {
		t.Fatalf("the chart starts at x %v, want the first sample at 0", first[0])
	}

	appendRows(t, st, 100, 400)
	c.Refresh()

	after := domainOf(t, c)
	if after[0] <= first[0] {
		t.Errorf("after the window slid the axis still starts at %v, want it past %v", after[0], first[0])
	}
	if after[1] <= first[1] {
		t.Errorf("the axis ends at %v, want it past the old end %v", after[1], first[1])
	}
}

func TestAnUnfollowedAxisKeepsEverythingItHasSeen(t *testing.T) {
	c, st := streaming(t, chart.ThemeFont(false), chart.Follow(false, false))

	first := domainOf(t, c)
	appendRows(t, st, 100, 400)
	c.Refresh()

	if after := domainOf(t, c); after[0] != first[0] {
		t.Errorf("an unfollowed axis moved its start from %v to %v", first[0], after[0])
	}
}

// A sliding window has nothing behind its tip: the rows that scrolled off were
// dropped. So a drag on a followed chart is ignored rather than allowed to
// strand the reader in front of data that no longer exists.
func TestDraggingAFollowedChartDoesNotTakeItOffTheData(t *testing.T) {
	c, st := streaming(t, chart.ThemeFont(false))

	before := domainOf(t, c)
	dragAcross(c)
	if after := domainOf(t, c); after != before {
		t.Errorf("a drag moved a followed axis from %v to %v", before, after)
	}

	appendRows(t, st, 100, 400)
	c.Refresh()
	if after := domainOf(t, c); after[0] <= before[0] {
		t.Errorf("after a drag the chart stopped following: %v", after)
	}
}

func TestZoomingAFollowedChartDoesNotTakeItOffTheData(t *testing.T) {
	c, win := streamingIn(t, chart.ThemeFont(false))

	before := domainOf(t, c)
	test.Scroll(win.Canvas(), fyne.NewPos(250, 150), 0, 4)
	if after := domainOf(t, c); after != before {
		t.Errorf("a wheel moved a followed axis from %v to %v", before, after)
	}
}

// With FollowPause the reader takes over instead, and the double click that
// resets the view hands the chart back to the data.
func TestWithFollowPauseAGrabStopsItFollowing(t *testing.T) {
	c, st := streaming(t, chart.ThemeFont(false), chart.FollowPause(true))

	dragAcross(c)
	panned := domainOf(t, c)

	appendRows(t, st, 100, 400)
	c.Refresh()
	if held := domainOf(t, c); held[0] != panned[0] {
		t.Errorf("a frame after a pan moved the axis from %v to %v", panned[0], held[0])
	}

	chart.PointerOf(c).DoubleTapped(&fyne.PointEvent{Position: fyne.NewPos(250, 150)})
	appendRows(t, st, 500, 700)
	c.Refresh()
	if resumed := domainOf(t, c); resumed[0] <= panned[0] {
		t.Errorf("after the reset the axis starts at %v, want it following past %v", resumed[0], panned[0])
	}
}

// A chart that follows nothing is panned as it always was.
func TestAChartWithoutAStreamIsStillPanned(t *testing.T) {
	c, _ := shown(t, fyne.NewSize(500, 300))

	before := domainOf(t, c)
	dragAcross(c)
	if after := domainOf(t, c); after == before {
		t.Errorf("a drag left a chart with no stream at %v", before)
	}
}

func dragAcross(c *chart.Chart) {
	chart.PointerOf(c).MouseDown(&desktop.MouseEvent{
		PointEvent: fyne.PointEvent{Position: fyne.NewPos(250, 150)},
		Button:     desktop.MouseButtonPrimary,
	})
	chart.PointerOf(c).Dragged(&fyne.DragEvent{
		PointEvent: fyne.PointEvent{Position: fyne.NewPos(310, 150)},
		Dragged:    fyne.NewDelta(60, 0),
	})
	chart.PointerOf(c).DragEnd()
}

// streaming is a chart over a sliding window, laid out and drawn once.
func streaming(t *testing.T, opts ...chart.Option) (*chart.Chart, *data.Stream) {
	t.Helper()
	c, _, st := streamingChart(t, opts...)
	return c, st
}

func streamingIn(t *testing.T, opts ...chart.Option) (*chart.Chart, fyne.Window) {
	t.Helper()
	c, win, _ := streamingChart(t, opts...)
	return c, win
}

func streamingChart(t *testing.T, opts ...chart.Option) (*chart.Chart, fyne.Window, *data.Stream) {
	t.Helper()
	st := data.NewStream("t", "y").Window(100)
	appendRows(t, st, 0, 100)

	p := figure.New(figure.Size(400, 250))
	p.Y(scale.Linear(scale.Domain(0, 2)))
	p.Add(geom.Line(st.Source(), geom.X("t"), geom.Y("y")))

	c := chart.New(p, append([]chart.Option{chart.Interactive(true)}, opts...)...)
	c.Stream(st)
	win := test.NewTempWindow(t, c)
	win.Resize(fyne.NewSize(500, 300))
	c.Resize(fyne.NewSize(500, 300))
	if err := c.Err(); err != nil {
		t.Fatalf("laying the chart out: %v", err)
	}
	return c, win, st
}

func appendRows(t *testing.T, st *data.Stream, from, to int) {
	t.Helper()
	for i := from; i < to; i++ {
		if err := st.Append(float64(i), 1+math.Sin(float64(i)/10)); err != nil {
			t.Fatalf("appending: %v", err)
		}
	}
}

// A followed axis reframes itself every frame, so any rounding in its framing
// shows up as the axis holding still while the data slides under it and then
// jumping a whole tick. The oldest samples leave the chart before the axis
// admits they are gone, which is the bug this pins.
func TestAFollowedAxisMovesWithEveryFrameRatherThanInSteps(t *testing.T) {
	c, st := streaming(t, chart.ThemeFont(false))

	var last [2]float64
	var held int
	for step := range 12 {
		appendRows(t, st, 100+step*5, 105+step*5)
		c.Refresh()
		if err := c.Err(); err != nil {
			t.Fatalf("frame %d: %v", step, err)
		}

		got := domainOf(t, c)
		if step > 0 && got == last {
			held++
			t.Errorf("frame %d left the axis at %v while the data moved on", step, got)
		}
		last = got
	}
	if held > 0 {
		t.Logf("the axis stood still on %d of 12 frames", held)
	}
}

// The window is what the axis shows: its span must stay the width of the
// window rather than growing with everything the chart has ever been given.
func TestAFollowedAxisIsAsWideAsTheWindow(t *testing.T) {
	c, st := streaming(t, chart.ThemeFont(false))

	appendRows(t, st, 100, 400)
	c.Refresh()

	got := domainOf(t, c)
	span := got[1] - got[0]
	// A window of 100 rows appended one x unit apart spans 99.
	if span < 90 || span > 110 {
		t.Errorf("the axis spans %v (%v), want about the window's 99", span, got)
	}
}
