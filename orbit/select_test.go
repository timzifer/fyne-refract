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
	// Four cameras, which is the figure the arrangement exists for: the
	// three-quarter view an author designs at, and the plan and two elevations
	// an engineering drawing has always had.
	c, _ := shown(t, fyne.NewSize(620, 300), fourViews(), orbit.Select(true))

	click(c, overSurface(t, c, 40, 300, 50, 140))
	sel := c.Selection()
	if len(sel) != 1 {
		t.Fatalf("clicking picked %d rows, want 1", len(sel))
	}

	// Every view drew the row, which is what the rings are placed from.
	if c.ViewCount() != 4 {
		t.Fatalf("the figure has %d views, want 4", c.ViewCount())
	}
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

// A scene hides its own far side, and a ring over a point on the far side has
// to say so — otherwise the reader takes a position off the near face that is
// not the one they picked.
func TestARingOnTheFarSideOfTheSurfaceIsMarkedHidden(t *testing.T) {
	c, _ := shown(t, fyne.NewSize(500, 300), plot(), orbit.Select(true))

	// Find a row the surface is currently in front of, and one it is not.
	// Which rows those are is a fact about the camera, so they are read out of
	// the frame that was just drawn rather than guessed at.
	idx := c.Live().Index()
	var behind, plain int = -1, -1
	for _, r := range idx.RowsOf(0, 0, nil) {
		if !r.Deep {
			continue
		}
		h, ok := idx.At(r.At, 0)
		if !ok || !h.Deep {
			continue
		}
		if h.Depth < r.Depth-1e-6 {
			if behind < 0 {
				behind = r.Row
			}
		} else if plain < 0 {
			plain = r.Row
		}
	}
	if behind < 0 || plain < 0 {
		t.Fatalf("the scene has no row behind another (%d) or none in the open (%d)", behind, plain)
	}

	c.SetSelection(fynefigure.Selection{{View: 0, Layer: 0, Row: behind}})
	if total, hidden := orbit.SelectionRings(c); total == 0 || hidden == 0 {
		t.Errorf("a row on the far side drew %d rings of which %d hidden, want at least one of each",
			total, hidden)
	}

	c.SetSelection(fynefigure.Selection{{View: 0, Layer: 0, Row: plain}})
	if total, hidden := orbit.SelectionRings(c); total == 0 || hidden != 0 {
		t.Errorf("a row in the open drew %d rings of which %d hidden, want none hidden", total, hidden)
	}
}

// Turning the scene changes which rows are hidden, and the rings follow —
// because they are resolved while the frame is drawn rather than when the
// selection changed.
func TestTurningTheSceneChangesWhichRingsAreHidden(t *testing.T) {
	c, _ := shown(t, fyne.NewSize(500, 300), plot(), orbit.Select(true))

	idx := c.Live().Index()
	rows := idx.RowsOf(0, 0, nil)
	if len(rows) == 0 {
		t.Fatal("the scene reported no rows")
	}
	sel := make(fynefigure.Selection, 0, len(rows))
	for _, r := range rows {
		sel = append(sel, fynefigure.Ref{View: 0, Layer: 0, Row: r.Row})
	}
	c.SetSelection(sel)

	seen := map[int]bool{}
	for range 8 {
		drag(c, fyne.NewPos(250, 150), fyne.NewDelta(30, 0))
		_, hidden := orbit.SelectionRings(c)
		seen[hidden] = true
	}
	if len(seen) < 2 {
		t.Errorf("turning the scene right round left the hidden count at %v throughout", seen)
	}
}

// A ring must not flicker between solid and dashed while the scene is dragged.
//
// It is the reason the occlusion test asks about the ring rather than about one
// pixel: a surface's cells overlap on screen wherever it is steep, so the cell
// next to the marked one covers its centroid at a grazing angle and is a hair
// nearer — true, not what a reader means by "behind", and decided by a fraction
// of a degree of turn.
func TestARingDoesNotFlickerWhileTheSceneTurns(t *testing.T) {
	c, _ := shown(t, fyne.NewSize(500, 300), plot(), orbit.Select(true))
	rows := c.Live().Index().RowsOf(0, 0, nil)
	if len(rows) == 0 {
		t.Fatal("the scene reported no rows")
	}
	c.SetSelection(fynefigure.Selection{{View: 0, Layer: 0, Row: rows[len(rows)/2].Row}})

	last, changes := -1, 0
	for range 120 {
		drag(c, fyne.NewPos(250, 150), fyne.NewDelta(1, 0))
		_, hidden := orbit.SelectionRings(c)
		if last >= 0 && hidden != last {
			changes++
		}
		last = hidden
	}
	// A turn of a hundred and twenty pixels genuinely takes a point behind the
	// surface and out again a few times. Many more than that is flicker.
	if changes > 6 {
		t.Errorf("one ring changed between solid and dashed %d times over 120 one-pixel drags", changes)
	}
}

// The same camera drawn twice gives the same answer, which is the floor under
// the test above: an answer that moved between two identical frames would be
// flicker nothing could damp.
func TestRedrawingTheSameSceneGivesTheSameRings(t *testing.T) {
	c, _ := shown(t, fyne.NewSize(500, 300), plot(), orbit.Select(true))
	rows := c.Live().Index().RowsOf(0, 0, nil)
	sel := make(fynefigure.Selection, 0, len(rows))
	for _, r := range rows {
		sel = append(sel, fynefigure.Ref{View: 0, Layer: 0, Row: r.Row})
	}
	c.SetSelection(sel)

	wantTotal, wantHidden := orbit.SelectionRings(c)
	if wantTotal == 0 {
		t.Fatal("the selection drew no rings")
	}
	for i := range 8 {
		c.Redraw()
		if total, hidden := orbit.SelectionRings(c); total != wantTotal || hidden != wantHidden {
			t.Fatalf("redraw %d drew %d rings of which %d hidden, want %d and %d",
				i, total, hidden, wantTotal, wantHidden)
		}
	}
}
