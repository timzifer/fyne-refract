package orbit_test

import (
	"math"
	"strconv"
	"testing"

	"fyne.io/fyne/v2"
	"github.com/timzifer/figure"
	"github.com/timzifer/figure/geom"
	"github.com/timzifer/figure/three"
	fynefigure "github.com/timzifer/fyne-figure"
	"github.com/timzifer/fyne-figure/orbit"
)

// A flat chart beside the scene makes refs that name *its* layer and row. Only
// the key crosses, so the scene has to find the row that answers to it — and
// then ring it, which it cannot do from a layer and a row that are not its own.
func TestASelectionFromAnotherChartIsPlacedByItsKey(t *testing.T) {
	// Select, as a chart linked to another has: it tracks the rows a ring is
	// placed by.
	c, _ := shown(t, fyne.NewSize(500, 300), keyedPlot(), orbit.Select(true))

	// Layer 7 and row 3 of some other chart's table: meaningless here.
	c.SetSelection(fynefigure.Selection{{Key: "r42", View: 0, Layer: 7, Row: 3}})

	got := c.Selection()
	if len(got) != 1 {
		t.Fatalf("the selection holds %d refs, want the one put there", len(got))
	}
	if got[0].Key != "r42" || got[0].Layer != 0 || got[0].Row != 42 {
		t.Errorf("the ref became key %q layer %d row %d, want key r42 at layer 0 row 42",
			got[0].Key, got[0].Layer, got[0].Row)
	}
	if total, _ := orbit.SelectionRings(c); total == 0 {
		t.Error("the row the key names was found and not ringed")
	}
}

// A key nothing in the scene answers to is kept as it came. The scene cannot
// draw it, and dropping it would lose it on the way back to the chart it came
// from.
func TestAKeyTheSceneDoesNotKnowIsKept(t *testing.T) {
	// Select, as a chart linked to another has: it tracks the rows a ring is
	// placed by.
	c, _ := shown(t, fyne.NewSize(500, 300), keyedPlot(), orbit.Select(true))

	in := fynefigure.Selection{{Key: "nobody", View: 0, Layer: 7, Row: 3}}
	c.SetSelection(in)

	if got := c.Selection(); !got.Equal(in) {
		t.Errorf("the selection became %+v, want %+v unchanged", got, in)
	}
	if total, _ := orbit.SelectionRings(c); total != 0 {
		t.Errorf("%d rings were drawn for a key the scene does not have", total)
	}
}

// keyedPlot is the test scene with a key column naming every row "r" and its
// index.
func keyedPlot() *three.Plot {
	const n = 10
	xs := make([]float64, 0, n*n)
	ys := make([]float64, 0, n*n)
	zs := make([]float64, 0, n*n)
	ids := make([]string, 0, n*n)
	for j := range n {
		for i := range n {
			x := -2 + 4*float64(i)/(n-1)
			y := -2 + 4*float64(j)/(n-1)
			xs, ys, zs = append(xs, x), append(ys, y), append(zs, math.Sin(x)*math.Cos(y))
			ids = append(ids, "r"+strconv.Itoa(len(ids)))
		}
	}
	src := figure.NewTable().Float64("x", xs).Float64("y", ys).Float64("z", zs).String("id", ids)
	sc := three.NewScene(three.XTitle("x"), three.YTitle("y"), three.ZTitle("z")).
		Add(three.Surface(src, geom.X("x"), geom.Y("y"), geom.Z("z"),
			geom.KeyBy("id"), geom.Label("response")))
	return three.New(three.Size(500, 300)).Scene(sc)
}
