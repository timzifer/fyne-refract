package orbit_test

import (
	"image/color"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/test"
	"github.com/timzifer/fyne-figure/orbit"
)

// Fyne sends an event to whatever implements the interface for it, wanted or
// not, so the widget itself must carry none of them — interactive or not.
func TestTheWidgetItselfTakesNoPointer(t *testing.T) {
	test.NewTempApp(t)
	var o fyne.CanvasObject = orbit.New(plot(), orbit.Interactive(true))

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
}

func TestASceneStaysStillUntilAsked(t *testing.T) {
	c := orbit.New(plot(), orbit.FrameInterval(-1))
	if c.Interactive() {
		t.Fatal("a chart nobody made interactive says it is")
	}
	tall := canvas.NewRectangle(color.Transparent)
	tall.SetMinSize(fyne.NewSize(10, 1000))
	scroll := container.NewVScroll(container.NewVBox(c, tall))
	win := test.NewTempWindow(t, scroll)
	win.Resize(fyne.NewSize(500, 300))
	if c.Live() == nil {
		t.Fatal("the scene in the scroll container was not laid out")
	}

	// Over the scene, which is at the top of the scrolled content.
	at := fyne.NewPos(250, 80)
	home := c.Camera(0)

	test.Drag(win.Canvas(), at, 60, 0)
	test.Scroll(win.Canvas(), at, 0, -20)
	if c.Camera(0) != home {
		t.Error("a drag and the wheel moved the camera of a still scene")
	}
	if scroll.Offset.Y == 0 {
		t.Error("the wheel over a still scene did not scroll the container around it")
	}

	c.SetInteractive(true)
	off := scroll.Offset
	test.Drag(win.Canvas(), at, 60, 0)
	turned := c.Camera(0)
	if turned == home {
		t.Error("a drag did not turn a scene made interactive")
	}
	test.Scroll(win.Canvas(), at, 0, 4)
	if scroll.Offset != off {
		t.Error("the container scrolled under a scene made interactive")
	}

	c.SetInteractive(false)
	turned = c.Camera(0)
	test.Drag(win.Canvas(), at, 60, 0)
	if c.Camera(0) != turned {
		t.Error("a drag turned a scene that was made still again")
	}
}
