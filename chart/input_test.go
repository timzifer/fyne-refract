package chart_test

import (
	"image/color"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/test"
	"github.com/timzifer/fyne_figure/chart"
)

// Fyne sends an event to whatever implements the interface for it, wanted or
// not, so a chart that only ignored the wheel would still have taken it. The
// widget itself must carry none of them.
func TestTheWidgetItselfTakesNoPointer(t *testing.T) {
	test.NewTempApp(t)
	var o fyne.CanvasObject = chart.New(plot(), chart.Interactive(true))

	if _, ok := o.(fyne.Scrollable); ok {
		t.Error("the chart is Scrollable, and would take the wheel from a scroll container around it")
	}
	if _, ok := o.(fyne.Draggable); ok {
		t.Error("the chart is Draggable")
	}
	if _, ok := o.(fyne.DoubleTappable); ok {
		t.Error("the chart is DoubleTappable")
	}
	if _, ok := o.(desktop.Hoverable); ok {
		t.Error("the chart is Hoverable")
	}
	if _, ok := o.(desktop.Mouseable); ok {
		t.Error("the chart is Mouseable")
	}
}

func TestAChartLeavesTheWheelToWhatIsAroundItUntilAsked(t *testing.T) {
	c, scroll, win := inScroll(t)
	if c.Interactive() {
		t.Fatal("a chart nobody made interactive says it is")
	}
	// Over the chart, which is at the top of the scrolled content.
	at := fyne.NewPos(250, 80)
	before := domainOf(t, c)

	test.Scroll(win.Canvas(), at, 0, -20)
	if scroll.Offset.Y == 0 {
		t.Error("the wheel over a still chart did not scroll the container around it")
	}
	if got := domainOf(t, c); got != before {
		t.Errorf("the wheel zoomed a still chart from %v to %v", before, got)
	}

	c.SetInteractive(true)
	off := scroll.Offset
	test.Scroll(win.Canvas(), at, 0, 4)
	zoomed := domainOf(t, c)
	if zoomed == before {
		t.Error("the wheel did not zoom a chart made interactive")
	}
	if scroll.Offset != off {
		t.Error("the container scrolled under a chart made interactive")
	}

	c.SetInteractive(false)
	off = scroll.Offset
	test.Scroll(win.Canvas(), at, 0, -20)
	if got := domainOf(t, c); got != zoomed {
		t.Errorf("the wheel zoomed a chart that was made still again, from %v to %v", zoomed, got)
	}
	if scroll.Offset == off {
		t.Error("the container did not get the wheel back from a chart made still again")
	}
}

func TestAStillChartShowsNoTooltip(t *testing.T) {
	c, _, win := inScroll(t)
	for x := float32(60); x < 460; x += 4 {
		test.MoveMouse(win.Canvas(), fyne.NewPos(x, 80))
	}
	if got := chart.TipText(c); got != "" {
		t.Errorf("hovering a still chart showed a tooltip reading %q", got)
	}
}

// inScroll puts a chart at the top of content taller than the window, which is
// where a still chart must not take the wheel from the container.
func inScroll(t *testing.T, opts ...chart.Option) (*chart.Chart, *container.Scroll, fyne.Window) {
	t.Helper()
	c := chart.New(plot(), append([]chart.Option{chart.ThemeFont(false), chart.FrameInterval(-1)}, opts...)...)
	tall := canvas.NewRectangle(color.Transparent)
	tall.SetMinSize(fyne.NewSize(10, 1000))
	scroll := container.NewVScroll(container.NewVBox(c, tall))

	win := test.NewTempWindow(t, scroll)
	win.Resize(fyne.NewSize(500, 300))
	if c.Live() == nil {
		t.Fatal("the chart in the scroll container was not laid out")
	}
	if err := c.Err(); err != nil {
		t.Fatalf("laying the chart out: %v", err)
	}
	return c, scroll, win
}
