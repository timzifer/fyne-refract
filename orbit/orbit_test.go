package orbit_test

import (
	"math"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/test"
	"github.com/timzifer/figure"
	"github.com/timzifer/figure/geom"
	"github.com/timzifer/figure/interact"
	"github.com/timzifer/figure/three"
	"github.com/timzifer/fyne_figure/orbit"
)

// What is asserted here is where the cameras are, not what the scene looks
// like: the widget's job is to turn Fyne's events into camera values, and the
// pixels those draw are figure's to test.

func TestASceneIsOpenedAtTheSizeItIsLaidOutAt(t *testing.T) {
	c, _ := shown(t, fyne.NewSize(500, 300), plot())

	if c.Live() == nil {
		t.Fatal("laying the widget out did not open a scene")
	}
	if w, h := c.Live().Size(); w != 500 || h != 300 {
		t.Errorf("the scene is %dx%d, want the size it was laid out at, 500x300", w, h)
	}
	if c.Target().Frames() == 0 {
		t.Error("laying the widget out painted nothing")
	}
}

func TestADragOrbitsTheScene(t *testing.T) {
	c, _ := shown(t, fyne.NewSize(500, 300), plot())
	before := c.Camera(0)

	drag(c, fyne.NewPos(250, 150), fyne.NewDelta(60, 0))

	// A drag to the right carries the camera left about the scene, so that
	// the side facing the reader follows the pointer.
	want := three.Orbit(before, -60*orbit.DefaultPerPixel, 0)
	if got := c.Camera(0); got != want {
		t.Errorf("after a drag of 60 pixels the azimuth is %.3f, want %.3f",
			got.Azimuth(), want.Azimuth())
	}
}

// A drag down tips the top of the scene toward the reader, which is the camera
// rising over it.
func TestDraggingDownRaisesTheCamera(t *testing.T) {
	c, _ := shown(t, fyne.NewSize(500, 300), plot())
	before := c.Camera(0)

	drag(c, fyne.NewPos(250, 150), fyne.NewDelta(0, 40))

	if got := c.Camera(0).Elevation(); got <= before.Elevation() {
		t.Errorf("dragging down moved the elevation from %.3f to %.3f, want it higher",
			before.Elevation(), got)
	}
}

func TestADragInStepsLandsWhereOneDragWould(t *testing.T) {
	c, _ := shown(t, fyne.NewSize(500, 300), plot())
	before := c.Camera(0)

	from := fyne.NewPos(250, 150)
	for range 4 {
		step := fyne.NewDelta(15, 0)
		from = from.Add(step)
		orbit.PointerOf(c).Dragged(&fyne.DragEvent{PointEvent: fyne.PointEvent{Position: from}, Dragged: step})
	}
	orbit.PointerOf(c).DragEnd()

	want := three.Orbit(before, -60*orbit.DefaultPerPixel, 0)
	if got := c.Camera(0).Azimuth(); math.Abs(got-want.Azimuth()) > 1e-9 {
		t.Errorf("four drags of 15 pixels left the azimuth at %.6f, want one of 60's %.6f",
			got, want.Azimuth())
	}
}

func TestTheWheelBringsTheSceneCloser(t *testing.T) {
	c, win := shown(t, fyne.NewSize(500, 300), plot())
	before := c.Camera(0).Zoom()

	test.Scroll(win.Canvas(), fyne.NewPos(250, 150), 0, 4)

	if got := c.Camera(0).Zoom(); got <= before {
		t.Errorf("scrolling up moved the zoom from %v to %v, want it larger", before, got)
	}
}

func TestADoubleClickGoesHome(t *testing.T) {
	c, win := shown(t, fyne.NewSize(500, 300), plot())
	home := c.Camera(0)

	drag(c, fyne.NewPos(250, 150), fyne.NewDelta(80, 30))
	test.Scroll(win.Canvas(), fyne.NewPos(250, 150), 0, 4)
	if c.Camera(0) == home {
		t.Fatal("the gestures moved nothing, so there is nothing to go home from")
	}

	orbit.PointerOf(c).DoubleTapped(&fyne.PointEvent{Position: fyne.NewPos(250, 150)})
	if got := c.Camera(0); got != home {
		t.Errorf("after a double click the camera is az %.3f el %.3f, want the author's az %.3f el %.3f",
			got.Azimuth(), got.Elevation(), home.Azimuth(), home.Elevation())
	}
}

func TestAResizeKeepsTheCamera(t *testing.T) {
	c, _ := shown(t, fyne.NewSize(500, 300), plot())
	drag(c, fyne.NewPos(250, 150), fyne.NewDelta(60, 20))
	turned := c.Camera(0)

	c.Resize(fyne.NewSize(640, 400))

	if w, h := c.Live().Size(); w != 640 || h != 400 {
		t.Errorf("after a resize the scene is %dx%d, want 640x400", w, h)
	}
	if got := c.Camera(0); got != turned {
		t.Error("a resize put the camera somewhere else")
	}
}

func TestADragTurnsOnlyTheViewItStartedIn(t *testing.T) {
	c, _ := shown(t, fyne.NewSize(600, 300), twoViews())
	left, right := c.Camera(0), c.Camera(1)

	// The right-hand cell.
	drag(c, fyne.NewPos(450, 170), fyne.NewDelta(50, 0))

	if c.Camera(0) != left {
		t.Error("a drag in the right-hand view turned the left-hand one")
	}
	if c.Camera(1) == right {
		t.Error("a drag in the right-hand view did not turn it")
	}
}

func TestTogetherTurnsEveryViewByTheSameAmount(t *testing.T) {
	c, _ := shown(t, fyne.NewSize(600, 300), twoViews(), orbit.Together(true))
	left, right := c.Camera(0), c.Camera(1)

	drag(c, fyne.NewPos(450, 170), fyne.NewDelta(50, 0))

	by := -50 * orbit.DefaultPerPixel
	if got, want := c.Camera(0), three.Orbit(left, by, 0); got != want {
		t.Error("turning together did not turn the left-hand view by the drag")
	}
	if got, want := c.Camera(1), three.Orbit(right, by, 0); got != want {
		t.Error("turning together did not turn the right-hand view by the drag")
	}
}

func TestOnCameraReportsAReaderAndNotACaller(t *testing.T) {
	c, _ := shown(t, fyne.NewSize(600, 300), twoViews())

	var views []int
	var last three.Camera
	c.OnCamera(func(i int, cam three.Camera) { views, last = append(views, i), cam })

	drag(c, fyne.NewPos(450, 170), fyne.NewDelta(50, 0))
	if len(views) != 1 || views[0] != 1 {
		t.Fatalf("a drag in the right-hand view reported views %v, want [1]", views)
	}
	if last != c.Camera(1) {
		t.Error("OnCamera reported a camera the view is not at")
	}

	views = nil
	if err := c.SetCamera(0, three.LookAt(three.Azimuth(1))); err != nil {
		t.Fatal(err)
	}
	if len(views) != 0 {
		t.Errorf("SetCamera reported views %v; a caller's move must not echo", views)
	}
	if got := c.Camera(0).Azimuth(); math.Abs(got-1) > 1e-9 {
		t.Errorf("SetCamera left the azimuth at %v, want 1", got)
	}
}

func TestHoveringFindsTheSurface(t *testing.T) {
	c, _ := shown(t, fyne.NewSize(500, 300), plot(), orbit.TrackRows(true))

	var hit interact.Hit
	var found bool
	c.OnHover(func(h interact.Hit, ok bool) {
		if ok && !found {
			hit, found = h, true
		}
	})

	// Sweep the middle of the cell: a surface fills most of it from the
	// author's angle, wherever the cube's margins happen to fall.
	for x := float32(150); x < 350 && !found; x += 8 {
		for y := float32(100); y < 220 && !found; y += 8 {
			orbit.PointerOf(c).MouseMoved(&desktop.MouseEvent{PointEvent: fyne.PointEvent{Position: fyne.NewPos(x, y)}})
		}
	}
	if !found {
		t.Fatal("no hover over the middle of the scene found anything")
	}
	if hit.Series != "response" {
		t.Errorf("the hover found series %q, want the surface's, %q", hit.Series, "response")
	}
	if hit.Row < 0 {
		t.Errorf("with rows tracked the hover found row %d, want a source row", hit.Row)
	}
}

func TestAClosedChartLetsGo(t *testing.T) {
	c, _ := shown(t, fyne.NewSize(500, 300), plot())
	if err := c.Close(); err != nil {
		t.Fatal(err)
	}
	if c.Live() != nil || c.Target() != nil {
		t.Error("a closed chart still holds its scene or its pixels")
	}
	// The pointer does not know the chart is gone.
	drag(c, fyne.NewPos(250, 150), fyne.NewDelta(60, 0))
}

func shown(t *testing.T, size fyne.Size, p *three.Plot, opts ...orbit.Option) (*orbit.Chart, fyne.Window) {
	t.Helper()
	// Every event drawn, so that a test does not depend on how fast the
	// machine running it rasterizes.
	c := orbit.New(p, append([]orbit.Option{orbit.FrameInterval(-1), orbit.Interactive(true)}, opts...)...)
	win := test.NewTempWindow(t, c)
	win.Resize(size)
	c.Resize(size)
	if err := c.Err(); err != nil {
		t.Fatalf("laying the scene out: %v", err)
	}
	return c, win
}

// drag sends a drag the way Fyne's desktop driver does: a position already
// past the press, and how far it came.
func drag(c *orbit.Chart, from fyne.Position, by fyne.Delta) {
	orbit.PointerOf(c).Dragged(&fyne.DragEvent{PointEvent: fyne.PointEvent{Position: from.Add(by)}, Dragged: by})
	orbit.PointerOf(c).DragEnd()
}

func plot() *three.Plot {
	return three.New(three.Size(500, 300)).Scene(scene())
}

func twoViews() *three.Plot {
	return three.New(three.Size(600, 300), three.Columns(2)).Scene(scene()).Add(
		three.View{Camera: three.Home()},
		three.View{Camera: three.LookAt(three.Elevation(1.2))},
	)
}

func scene() *three.Scene {
	const n = 10
	xs := make([]float64, 0, n*n)
	ys := make([]float64, 0, n*n)
	zs := make([]float64, 0, n*n)
	for j := range n {
		for i := range n {
			x := -2 + 4*float64(i)/(n-1)
			y := -2 + 4*float64(j)/(n-1)
			xs, ys, zs = append(xs, x), append(ys, y), append(zs, math.Sin(x)*math.Cos(y))
		}
	}
	src := figure.NewTable().Float64("x", xs).Float64("y", ys).Float64("z", zs)
	return three.NewScene(three.XTitle("x"), three.YTitle("y"), three.ZTitle("z")).
		Add(three.Surface(src, geom.X("x"), geom.Y("y"), geom.Z("z"), geom.Label("response")))
}
