package chart_test

import (
	"image"
	"testing"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/test"
	"github.com/timzifer/fyne_figure/chart"
)

// A chart being dragged is rasterized coarser than the screen and stretched by
// the graphics card, which is what keeps it following the pointer. When the
// gesture ends it is drawn properly again.

func TestADraggedChartIsRasterizedCoarserAndSharpensAfterwards(t *testing.T) {
	c, _ := shown(t, fyne.NewSize(500, 300), chart.Detail(0.5))

	full := buffer(t, c)
	if full.X != 500 || full.Y != 300 {
		t.Fatalf("a still chart rasterizes %v, want the widget's 500x300", full)
	}

	press(c, 400, 150)
	drag(c, 300, 150)
	coarse := buffer(t, c)
	if coarse.X != 250 || coarse.Y != 150 {
		t.Errorf("a dragged chart rasterizes %v, want half of 500x300", coarse)
	}

	chart.PointerOf(c).DragEnd()
	if after := buffer(t, c); after != full {
		t.Errorf("after the drag the chart rasterizes %v, want %v", after, full)
	}
}

// Softening a chart while it is dragged is a visible trade, so it is offered
// rather than given: a chart nobody configured is drawn at full resolution
// throughout.
func TestAChartIsDrawnAtFullResolutionUnlessAsked(t *testing.T) {
	c, _ := shown(t, fyne.NewSize(500, 300))

	full := buffer(t, c)
	press(c, 400, 150)
	drag(c, 300, 150)
	if coarse := buffer(t, c); coarse != full {
		t.Errorf("a chart with no Detail option rasterized %v while dragged, want %v", coarse, full)
	}
	chart.PointerOf(c).DragEnd()
}

func TestDetailCanBeTurnedOff(t *testing.T) {
	c, _ := shown(t, fyne.NewSize(500, 300), chart.Detail(1))

	full := buffer(t, c)
	press(c, 400, 150)
	drag(c, 300, 150)
	if coarse := buffer(t, c); coarse != full {
		t.Errorf("a chart told to keep its detail rasterized %v while dragged, want %v", coarse, full)
	}
	chart.PointerOf(c).DragEnd()
}

func TestAWheelSharpensOnceItStops(t *testing.T) {
	c, win := shown(t, fyne.NewSize(500, 300), chart.Detail(0.5))
	full := buffer(t, c)

	test.Scroll(win.Canvas(), fyne.NewPos(250, 150), 0, 4)
	if coarse := buffer(t, c); coarse == full {
		t.Fatalf("a zoom left the chart at %v, want it coarser", coarse)
	}

	// A wheel has no end to report, so the sharp frame is on a timer.
	deadline := time.Now().Add(2 * time.Second)
	for buffer(t, c) != full && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
	if after := buffer(t, c); after != full {
		t.Errorf("the chart stayed at %v after the wheel stopped, want %v", after, full)
	}
}

// A coarse frame must not be read back as a display with fewer pixels, which
// would leave the chart coarse for good.
func TestACoarseFrameIsNotMistakenForTheDisplay(t *testing.T) {
	c, _ := shown(t, fyne.NewSize(500, 300), chart.Detail(0.5))

	press(c, 400, 150)
	drag(c, 300, 150)

	// This is what the painter does with a frame smaller than it asked for.
	raster := c.Target().Object().(*canvas.Raster)
	raster.Generator(500, 300)
	c.Refresh()

	if got := buffer(t, c); got.X != 250 {
		t.Errorf("a painted coarse frame changed the rasterizer to %v", got)
	}
	chart.PointerOf(c).DragEnd()
	if after := buffer(t, c); after.X != 500 {
		t.Errorf("after the drag the chart rasterizes %v, want the full 500 wide", after)
	}
}

// buffer is the size of the pixel buffer the chart is currently rasterized at.
func buffer(t *testing.T, c *chart.Chart) image.Point {
	t.Helper()
	img := c.Target().Image()
	if img == nil {
		t.Fatal("the chart has no pixels")
	}
	return image.Pt(img.Bounds().Dx(), img.Bounds().Dy())
}
