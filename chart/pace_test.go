package chart_test

import (
	"testing"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/test"
	"github.com/timzifer/figure"
	"github.com/timzifer/fyne_figure/chart"
)

// Fyne's desktop driver drains the whole event queue in one pass, so a drag
// delivers events far faster than a chart of any size can be drawn. Drawing
// every one of them is asking for more work than there is time for, and the
// backlog never drains — the chart falls behind the cursor for as long as the
// drag lasts. So they are paced: the newest position wins and the rest are
// dropped.

func TestABurstOfDragsCostsAFewFramesRatherThanOnePerEvent(t *testing.T) {
	c, _ := shown(t, fyne.NewSize(500, 300), chart.FrameInterval(time.Second))

	before := frames(t, c)
	press(c, 400, 150)
	for i := range 100 {
		drag(c, 400-float32(i), 150)
	}
	painted := frames(t, c) - before

	if painted > 3 {
		t.Errorf("a hundred drag events painted %d frames, want a handful", painted)
	}
	if painted == 0 {
		t.Error("a hundred drag events painted nothing at all")
	}
}

// Dropping a pan loses nothing: figure pans by the distance from the last
// position it was told about, so the next position it hears covers the whole
// way, and the end of the gesture draws whatever is left over.
//
// The two do not land on the same float, and the difference is not this
// package's. figure converts a pan through float32 device coordinates once
// per step, so ninety-seven pans of one pixel land about 0.7 of a pixel away
// from one pan of ninety-seven — measured, and independent of any pacing. What
// is asserted here is therefore that the gesture ends where it was aimed, to
// within a pixel, rather than on an identical number.
func TestAPacedDragEndsWhereAnUnpacedOneDoes(t *testing.T) {
	paced, _ := shown(t, fyne.NewSize(500, 300), chart.FrameInterval(time.Second))
	press(paced, 400, 150)
	for i := 1; i <= 100; i++ {
		drag(paced, 400-float32(i), 150)
	}
	paced.DragEnd()

	// The same gesture with the pacing off, which is one frame per event.
	every, _ := shown(t, fyne.NewSize(500, 300), chart.FrameInterval(-1))
	press(every, 400, 150)
	for i := 1; i <= 100; i++ {
		drag(every, 400-float32(i), 150)
	}
	every.DragEnd()

	got, want := domainOf(t, paced), domainOf(t, every)
	if !withinAPixel(got, want, 500) {
		t.Errorf("the paced drag ended at %v, the unpaced one at %v", got, want)
	}
}

// The same for the wheel, where the deltas are added up rather than dropped.
func TestPacedZoomLandsWhereEveryNotchWould(t *testing.T) {
	paced, win := shown(t, fyne.NewSize(500, 300), chart.FrameInterval(time.Second))
	for range 20 {
		test.Scroll(win.Canvas(), fyne.NewPos(250, 150), 0, 1)
	}
	paced.MouseOut() // ends the gesture, drawing what is held back

	every, everyWin := shown(t, fyne.NewSize(500, 300), chart.FrameInterval(-1))
	for range 20 {
		test.Scroll(everyWin.Canvas(), fyne.NewPos(250, 150), 0, 1)
	}

	got, want := domainOf(t, paced), domainOf(t, every)
	if !withinAPixel(got, want, 500) {
		t.Errorf("the paced zoom landed at %v, notch by notch it lands at %v", got, want)
	}
}

// A hover draws nothing, so it is not paced and must stay immediate.
func TestHoveringIsNotPaced(t *testing.T) {
	c, win := shown(t, fyne.NewSize(500, 300), chart.FrameInterval(time.Second))

	var hovers int
	c.Plot().On(figure.Hover, func(figure.Event) { hovers++ })
	for i := range 20 {
		test.MoveMouse(win.Canvas(), fyne.NewPos(100+float32(i)*5, 150))
	}
	if hovers != 20 {
		t.Errorf("twenty pointer moves reported %d hovers, want all of them", hovers)
	}
}

// benchChart is a laid-out chart without a testing.T to hand.
func benchChart(b *testing.B, opts ...chart.Option) *chart.Chart {
	b.Helper()
	c := chart.New(benchPlot(), append([]chart.Option{chart.ThemeFont(false)}, opts...)...)
	win := test.NewWindow(c)
	b.Cleanup(win.Close)
	win.Resize(fyne.NewSize(900, 500))
	c.Resize(fyne.NewSize(900, 500))
	return c
}

func press(c *chart.Chart, x, y float32) {
	c.MouseDown(&desktop.MouseEvent{
		PointEvent: fyne.PointEvent{Position: fyne.NewPos(x, y)},
		Button:     desktop.MouseButtonPrimary,
	})
}

func drag(c *chart.Chart, x, y float32) {
	c.Dragged(&fyne.DragEvent{
		PointEvent: fyne.PointEvent{Position: fyne.NewPos(x, y)},
		Dragged:    fyne.NewDelta(-1, 0),
	})
}

// withinAPixel reports whether two domains differ by less than one pixel of a
// chart that wide, which is the resolution anything about a pointer gesture
// has.
func withinAPixel(a, b [2]float64, widthPx float64) bool {
	perPixel := (b[1] - b[0]) / widthPx
	return absOf(a[0]-b[0]) <= perPixel && absOf(a[1]-b[1]) <= perPixel
}

func absOf(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}

// What the pacing is worth: a burst of pointer events the size of one second
// of a real drag, timed with the pacing on and off.
//
//	go test -run='^$' -bench=Drag -benchtime=5x ./chart
//
// What a single drag frame costs, which is what a reader feels: the pacing
// decides how many are drawn, this decides how long each one takes.
//
//	go test -run='^$' -bench=DragFrame -benchtime=20x ./chart
func BenchmarkDragFrame(b *testing.B) {
	for _, detail := range []float32{1, 0.5} {
		name := "full"
		if detail != 1 {
			name = "coarse"
		}
		b.Run(name, func(b *testing.B) {
			c := benchChart(b, chart.FrameInterval(-1), chart.Detail(detail))
			press(c, 880, 250)
			drag(c, 860, 250) // past the click slop, and coarse if it is going to be

			b.ResetTimer()
			for i := 0; b.Loop(); i++ {
				drag(c, 860-float32(i%200), 250)
			}
			b.StopTimer()
			c.DragEnd()
		})
	}
}

func BenchmarkDragBurst(b *testing.B) {
	for _, paced := range []bool{true, false} {
		name := "unpaced"
		if paced {
			name = "paced"
		}
		b.Run(name, func(b *testing.B) {
			opt := chart.FrameInterval(-1)
			if paced {
				opt = chart.FrameInterval(0) // adaptive: the last frame's cost
			}
			c := benchChart(b, opt, chart.Detail(1))

			b.ResetTimer()
			for b.Loop() {
				press(c, 400, 150)
				for i := 1; i <= 100; i++ {
					drag(c, 400-float32(i), 150)
				}
				c.DragEnd()
			}
		})
	}
}
