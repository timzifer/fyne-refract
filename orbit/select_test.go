package orbit_test

import (
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver/desktop"
	"github.com/timzifer/figure/ir"
	fynefigure "github.com/timzifer/fyne_figure"
	"github.com/timzifer/fyne_figure/orbit"
)

// click presses and releases without moving, which is what tells a click from
// the beginning of a turn.
func click(c *orbit.Chart, pos fyne.Position) {
	ev := &desktop.MouseEvent{PointEvent: fyne.PointEvent{Position: pos}, Button: desktop.MouseButtonPrimary}
	orbit.PointerOf(c).MouseDown(ev)
	orbit.PointerOf(c).MouseUp(ev)
}

// overSurface finds a position over the surface by sweeping the middle of the
// first cell, the way the hover test does.
func overSurface(t *testing.T, c *orbit.Chart, x0, x1, y0, y1 float32) fyne.Position {
	t.Helper()
	for x := x0; x < x1; x += 6 {
		for y := y0; y < y1; y += 6 {
			pos := fyne.NewPos(x, y)
			orbit.PointerOf(c).MouseMoved(&desktop.MouseEvent{PointEvent: fyne.PointEvent{Position: pos}})
			if _, ok := c.Live().Index().At(ir.Point{X: pos.X, Y: pos.Y}, 6); ok {
				return pos
			}
		}
	}
	t.Fatal("nothing was found anywhere over the middle of the scene")
	return fyne.Position{}
}

func TestClickingAMarkPicksItsRow(t *testing.T) {
	c, _ := shown(t, fyne.NewSize(500, 300), plot(), orbit.Select(true))

	var told fynefigure.Selection
	calls := 0
	c.OnSelect(func(s fynefigure.Selection) { told, calls = s, calls+1 })

	click(c, overSurface(t, c, 150, 350, 100, 220))

	sel := c.Selection()
	if len(sel) != 1 {
		t.Fatalf("clicking a mark picked %d rows, want 1", len(sel))
	}
	if sel[0].Row < 0 {
		t.Errorf("the picked row is %d; a scene with Select must track rows", sel[0].Row)
	}
	if calls != 1 || !told.Equal(sel) {
		t.Errorf("the handler was told %v after %d calls, want %v after 1", told, calls, sel)
	}
	if err := c.Err(); err != nil {
		t.Errorf("drawing the scene with a selection: %v", err)
	}
}

// A scene that was not asked to select does not.
func TestASceneDoesNotSelectUnlessAsked(t *testing.T) {
	c, _ := shown(t, fyne.NewSize(500, 300), plot(), orbit.TrackRows(true))

	click(c, overSurface(t, c, 150, 350, 100, 220))
	if got := c.Selection(); len(got) != 0 {
		t.Errorf("a scene without Select picked %v", got)
	}
}

// A drag turns the scene and picks nothing. The two gestures start the same
// way, and a turn that also picked a row would pick whatever happened to be
// under the press.
func TestADragPicksNothing(t *testing.T) {
	c, _ := shown(t, fyne.NewSize(500, 300), plot(), orbit.Select(true))

	from := overSurface(t, c, 150, 350, 100, 220)
	orbit.PointerOf(c).MouseDown(&desktop.MouseEvent{
		PointEvent: fyne.PointEvent{Position: from}, Button: desktop.MouseButtonPrimary,
	})
	drag(c, from, fyne.NewDelta(50, 20))
	orbit.PointerOf(c).MouseUp(&desktop.MouseEvent{
		PointEvent: fyne.PointEvent{Position: from.Add(fyne.NewDelta(50, 20))}, Button: desktop.MouseButtonPrimary,
	})

	if got := c.Selection(); len(got) != 0 {
		t.Errorf("a drag picked %v", got)
	}
}

// A click on nothing clears the selection.
func TestClickingNothingClearsTheSelection(t *testing.T) {
	c, _ := shown(t, fyne.NewSize(500, 300), plot(), orbit.Select(true))

	click(c, overSurface(t, c, 150, 350, 100, 220))
	if len(c.Selection()) != 1 {
		t.Fatal("the row was not picked to begin with")
	}
	click(c, fyne.NewPos(3, 3))
	if got := c.Selection(); len(got) != 0 {
		t.Errorf("clicking outside every mark left %v picked", got)
	}
}

// The point of the whole arrangement: one click, and the row is marked in
// every view rather than in the one it was picked in.
func TestOneClickRingsTheRowInEveryView(t *testing.T) {
	c, _ := shown(t, fyne.NewSize(600, 300), twoViews(), orbit.Select(true))

	click(c, overSurface(t, c, 60, 290, 60, 240))
	sel := c.Selection()
	if len(sel) != 1 {
		t.Fatalf("clicking picked %d rows, want 1", len(sel))
	}

	// Both views drew the row, which is what the rings are placed from.
	for view := range c.ViewCount() {
		if _, ok := c.Live().Index().Locate(view, sel[0].Layer, sel[0].Row); !ok {
			t.Errorf("view %d did not draw the picked row, so it can carry no ring", view)
		}
	}
	if c.Live().CurrentOverlay() == nil {
		t.Error("a scene with a selection has no overlay installed to draw it")
	}
}

// A selection put into the scene is not one the reader made, so it reports
// nothing back — which is what keeps a link from running away.
func TestSettingASelectionDoesNotReportIt(t *testing.T) {
	c, _ := shown(t, fyne.NewSize(500, 300), plot(), orbit.Select(true))

	calls := 0
	c.OnSelect(func(fynefigure.Selection) { calls++ })

	c.SetSelection(fynefigure.Selection{{Key: "k", Layer: -1, Row: -1}})
	if calls != 0 {
		t.Errorf("SetSelection called the handler %d times", calls)
	}
	if got := c.Selection(); len(got) != 1 {
		t.Errorf("the scene holds %v, want the row it was given", got)
	}
}

// A scene with nothing picked installs no overlay. An installed one gives up
// the partial repaint that makes a several-view figure affordable to drag, and
// an empty one would give it up for nothing.
func TestAnEmptySelectionInstallsNoOverlay(t *testing.T) {
	c, _ := shown(t, fyne.NewSize(500, 300), plot(), orbit.Select(true))

	if c.Live().CurrentOverlay() != nil {
		t.Fatal("a scene with nothing picked has an overlay installed")
	}
	click(c, overSurface(t, c, 150, 350, 100, 220))
	if c.Live().CurrentOverlay() == nil {
		t.Fatal("picking a row installed no overlay")
	}
	c.SetSelection(nil)
	if c.Live().CurrentOverlay() != nil {
		t.Error("clearing the selection left an overlay installed")
	}
}

// A selection draws, and clearing it puts the scene back exactly.
func TestASelectionIsDrawn(t *testing.T) {
	c, _ := shown(t, fyne.NewSize(500, 300), plot(), orbit.Select(true))

	pos := overSurface(t, c, 150, 350, 100, 220)
	before := pixels(t, c)
	click(c, pos)
	if got := pixels(t, c); got == before {
		t.Error("picking a row changed nothing on screen")
	}
	c.SetSelection(nil)
	if got := pixels(t, c); got != before {
		t.Error("clearing the selection did not put the scene back")
	}
}

func pixels(t *testing.T, c *orbit.Chart) uint64 {
	t.Helper()
	img := c.Target().Image()
	if img == nil {
		t.Fatal("the scene has no pixels")
	}
	var sum uint64
	b := img.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y += 3 {
		for x := b.Min.X; x < b.Max.X; x += 3 {
			r, g, bl, a := img.At(x, y).RGBA()
			sum = sum*31 + uint64(r) + uint64(g)<<8 + uint64(bl)<<16 + uint64(a)<<24
		}
	}
	return sum
}
