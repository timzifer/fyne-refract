package main

import (
	"fmt"
	"math"
	"strconv"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
	"github.com/timzifer/figure"
	"github.com/timzifer/figure/geom"
	"github.com/timzifer/figure/palette"
	"github.com/timzifer/figure/scale"
	"github.com/timzifer/figure/stat"
	"github.com/timzifer/figure/three"
	fynefigure "github.com/timzifer/fyne_figure"
	"github.com/timzifer/fyne_figure/chart"
	"github.com/timzifer/fyne_figure/orbit"
)

// The field's range, chosen once and used by everything that colours it or
// draws a level through it.
const (
	gainLo = -12.0
	gainHi = 6.0
)

// contourTab is one field read two ways: a plan to take numbers off, and a
// shape to see what they are doing. It is figure's examples/contour as a pair
// of widgets.
//
// Three things are shared, and each is what makes the pair one picture rather
// than two that agree by luck:
//
//   - one colour scale, pinned with scale.ColorDomain, so a colour is one
//     number in the heatmap, on the surface and on the floor;
//   - one list of levels, so the white lines over the heatmap and the lines on
//     the scene's floor are the same lines;
//   - one key column, so a cell clicked in either chart is ringed in both. The
//     two charts are drawn from one table here, but the key is what crosses —
//     the link would work as well from two tables that named their rows alike.
func contourTab() fyne.CanvasObject {
	ramp := scale.Sequential(palette.Viridis, scale.ColorDomain(gainLo, gainHi))
	levels := stat.Levels(gainLo, gainHi, 9)
	field := gainField()

	p := figure.New(
		figure.Responsive(true),
		figure.Size(460, 420),
		figure.Title("Plan"),
		figure.XTitle("bias (V)"),
		figure.YTitle("drive (dBm)"),
	)
	p.X(scale.Linear())
	p.Y(scale.Linear())
	p.Add(
		// BarWidth(1) closes the gutters between cells: a sampled field is a
		// continuous thing, and gaps would say the measurement stops between
		// the samples.
		geom.Rect(field.src, geom.X("bias"), geom.Y("drive"),
			geom.ColorBy("gain", ramp), geom.BarWidth(1),
			geom.KeyBy("cell"), geom.Label("gain")),
		// White rather than the ramp: over a filled field a coloured line
		// competes with the fill it is meant to be read against.
		geom.Contour(field.src, geom.X("bias"), geom.Y("drive"), geom.Z("gain"),
			geom.Levels(levels...), geom.Color(palette.White), geom.Width(1)),
	)
	plan := chart.New(p, chart.Interactive(true), chart.Select(true))

	sc := three.NewScene(
		three.XTitle("bias (V)"),
		three.YTitle("drive (dBm)"),
		three.ZTitle("gain (dB)"),
	).
		Z(scale.Linear(scale.Nice())).
		Add(
			// The floor: the same levels, traced once, in the ramp's colours.
			three.Contour(field.src, geom.X("bias"), geom.Y("drive"), geom.Z("gain"),
				geom.Levels(levels...), geom.ColorBy("gain", ramp)),
			three.Surface(field.src, geom.X("bias"), geom.Y("drive"), geom.Z("gain"),
				geom.ColorBy("gain", ramp), geom.KeyBy("cell"), geom.Label("gain")),
		)
	shape := orbit.New(
		three.New(three.Size(460, 420), three.Title("Shape")).Scene(sc),
		orbit.Interactive(true), orbit.Select(true),
	)

	hint := "Click a cell in either chart to ring it in both. Drag the shape to turn it."
	status := widget.NewLabel(hint)
	show := func(sel fynefigure.Selection) {
		if len(sel) == 0 {
			status.SetText(hint)
			return
		}
		b, d, g, ok := field.at(sel[0].Key)
		if !ok {
			status.SetText(hint)
			return
		}
		status.SetText(fmt.Sprintf("bias %.2f V   drive %.1f dBm   gain %.2f dB", b, d, g))
	}

	// The link, one line each way. SetSelection reports nothing back, so it
	// does not loop.
	plan.OnSelect(func(sel fynefigure.Selection) { shape.SetSelection(sel); show(sel) })
	shape.OnSelect(func(sel fynefigure.Selection) { plan.SetSelection(sel); show(sel) })

	clear := widget.NewButton("Clear selection", func() {
		plan.SetSelection(nil)
		shape.SetSelection(nil)
		show(nil)
	})
	home := widget.NewButton("Home", func() { _ = shape.Home() })

	charts := container.NewGridWithColumns(2, plan, shape)
	return container.NewBorder(nil,
		container.NewVBox(status, container.NewHBox(home, clear)), nil, nil, charts)
}

// field is the sampled gain and the columns it was built from, kept so that a
// selection's key can be read back as numbers.
type field struct {
	src               figure.Source
	bias, drive, gain []float64
	index             map[string]int
}

// at reads the sample one key names.
func (f field) at(key string) (bias, drive, gain float64, ok bool) {
	i, ok := f.index[key]
	if !ok {
		return 0, 0, 0, false
	}
	return f.bias[i], f.drive[i], f.gain[i], true
}

// gainField is a device's small-signal gain over a bias and a drive level: a
// ridge that runs diagonally and rolls off at both ends. It is figure's
// examples/contour field, with a key column naming each cell.
func gainField() field {
	const n = 40
	f := field{index: make(map[string]int, n*n)}
	cells := make([]string, 0, n*n)
	for j := range n {
		for i := range n {
			b := -1 + 2*float64(i)/(n-1)
			d := -20 + 30*float64(j)/(n-1)
			key := strconv.Itoa(i) + "," + strconv.Itoa(j)
			f.index[key] = len(cells)
			cells = append(cells, key)
			f.bias, f.drive = append(f.bias, b), append(f.drive, d)
			f.gain = append(f.gain, 4-0.02*(d+6)*(d+6)-6*(b-0.1)*(b-0.1)+1.5*b*math.Cos(d/6))
		}
	}
	f.src = figure.NewTable().
		Float64("bias", f.bias).Float64("drive", f.drive).Float64("gain", f.gain).
		String("cell", cells)
	return f
}
