// Command demo shows figure charts in a Fyne window.
//
//	cd cmd/demo && go run .
//
// It is a module of its own so that it can import the GPU tier, which is
// nested inside the widget's module and so outside its graph. See go.mod
// beside this file.
//
// Two tabs. The first is a still signal: hover to see what is under the
// pointer, drag to pan, turn the wheel to zoom about it, double click to go
// back to the whole thing, and resize the window to see the chart laid out
// again. "Export" writes exactly what is on screen to a PNG, through the same
// rasterizer that drew it.
//
// The second is live: a producer appends on its own goroutine and the chart is
// repainted on a timer. It hovers, and it does not drag or zoom — its axis
// follows the window, and a sliding window has nothing behind its tip to pan
// to. chart.FollowPause is the other choice; see the Follow tab's comment.
//
// The third is the interaction figure grew in v1.7 and this widget wires:
// two charts that move together, a legend whose rows can be clicked off, a
// crosshair painted over the finished chart, and a drag that marks out rows
// instead of panning. None of it is on by default — a legend that always
// toggled would be wrong for one that selects rather than filters — so this
// tab is also the list of the switches.
//
// The fourth is a scene in three dimensions, which figure grew in v0.9: one
// surface seen from four cameras — the three-quarter view an author designs at
// and the plan and two elevations an engineering drawing has always had. Drag a
// view to turn it, turn the wheel over it to bring it closer, double click to
// put them all back. Click a point and it is ringed in all four at once, which
// is what several views are for: identifying a measurement from one angle and
// finding it again from the others.
//
// The fifth is the same four cameras as four widgets, with the glue that made
// them one figure written out in the program: a pick ringed in all four, a turn
// that turns all four, and the others drawn at half resolution while one is
// turned so that the one under the pointer keeps up. See cameras.go.
//
// The sixth is one field read two ways off one colour scale: a heatmap with its
// isolines to take numbers off, and the surface with the same isolines on its
// floor. A cell clicked in either is ringed in both. See contour.go.
package main

import (
	"fmt"
	"log"
	"math"
	"math/rand/v2"
	"os"
	"path/filepath"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
	"github.com/timzifer/figure"
	ggbackend "github.com/timzifer/figure/backend/gg"
	"github.com/timzifer/figure/data"
	"github.com/timzifer/figure/geom"
	"github.com/timzifer/figure/interact"
	"github.com/timzifer/figure/palette"
	"github.com/timzifer/figure/scale"
	"github.com/timzifer/figure/three"
	fynefigure "github.com/timzifer/fyne_figure"
	"github.com/timzifer/fyne_figure/chart"
	_ "github.com/timzifer/fyne_figure/gpu"
	"github.com/timzifer/fyne_figure/orbit"
)

func main() {
	a := app.New()
	w := a.NewWindow("figure — Fyne")
	w.Resize(fyne.NewSize(960, 620))

	live, animate := liveTab()
	liveItem := container.NewTabItem("Live", live)

	tabs := container.NewAppTabs(
		container.NewTabItem("Signal", signalTab()),
		liveItem,
		container.NewTabItem("Interact", interactTab()),
		container.NewTabItem("3D", sceneTab()),
		container.NewTabItem("3D × 4", camerasTab()),
		container.NewTabItem("Contour", contourTab()),
	)
	// A chart nobody is looking at should not be drawn. Both tabs share one
	// goroutine — Fyne's — so a hidden chart repainting twenty times a second
	// is time the visible one does not get.
	var stop func()
	tabs.OnSelected = func(item *container.TabItem) {
		if stop != nil {
			stop()
			stop = nil
		}
		if item == liveItem {
			stop = animate()
		}
	}

	w.SetContent(tabs)
	w.ShowAndRun()
}

// signalTab is a chart with something to point at, and the two controls a
// reader looks for.
func signalTab() fyne.CanvasObject {
	p := figure.New(
		figure.Responsive(true),
		figure.Size(900, 480),
		figure.Title("Signal"),
		figure.XTitle("t"),
		figure.YTitle("amplitude"),
	)
	p.Add(
		geom.Line(signal(4000), geom.X("t"), geom.Y("signal"),
			geom.Color(palette.SkyBlue), geom.Label("signal")),
	)

	// chart.Detail(0.5) here would draw the chart at half resolution while it
	// is dragged and sharpen it afterwards, which is worth about three times
	// the frame rate on the CPU rasterizer. It is left off so that the demo
	// shows what the widget does by default.
	c := chart.New(p, chart.Interactive(true), chart.TrackRows(true))

	status := widget.NewLabel("Hover the chart.")
	p.On(figure.Hover, func(ev figure.Event) {
		if !ev.Found {
			status.SetText("Hover the chart.")
			return
		}
		status.SetText(fmt.Sprintf("%s   x %.4g   y %.4g   row %d",
			ev.Series(), ev.Hit.X, ev.Hit.Y, ev.Hit.Row))
	})

	reset := widget.NewButton("Autoscale", func() {
		if err := c.Autoscale(); err != nil {
			status.SetText("autoscale: " + err.Error())
		}
	})
	export := widget.NewButton("Export PNG", func() {
		path := filepath.Join(os.TempDir(), "figure-demo.png")
		// The same plot through the same rasterizer: what lands in the file is
		// what is on screen, which is the whole point of drawing it this way.
		if err := p.Render(ggbackend.PNG(path)); err != nil {
			status.SetText("export: " + err.Error())
			return
		}
		status.SetText("wrote " + path)
	})

	controls := container.NewHBox(reset, export)
	return container.NewBorder(nil, container.NewVBox(status, controls), nil, nil, c)
}

// interactTab is two charts of the same table, linked, with everything a
// pointer can be made to mean switched on.
func interactTab() fyne.CanvasObject {
	src := signal(2000)

	// Two charts of the same rows, so that a view handed from one to the other
	// means the same thing in both.
	top := chart.New(plotOf(src, "Signal — drag to select"),
		chart.Interactive(true),
		chart.LegendToggle(true),
		chart.DragMode(figure.DragSelects),
	)
	bottom := chart.New(plotOf(src, "The same rows, linked"),
		chart.Interactive(true),
		chart.LegendToggle(true),
	)

	// The link, one line each way. It does not loop: SetView is not a reader
	// moving anything, and reports nothing back.
	top.OnViewChange(func(v figure.View) { _ = bottom.SetView(v) })
	bottom.OnViewChange(func(v figure.View) { _ = top.SetView(v) })

	// A crosshair is the cheapest thing a reader can be given for "which value
	// is this". It is installed once and then moved: the overlay is a pointer
	// whose fields the handler writes.
	cross := &figure.Crosshair{}
	top.Overlay(cross)

	status := widget.NewLabel("Drag a rectangle over the top chart. Click a legend row to hide a series.")
	top.Plot().On(figure.Hover, func(ev figure.Event) {
		cross.At, cross.Show = ev.Hit.At, ev.Found && !ev.Hit.Kind.Guides()
	})
	// A selection is one event per layer under the rectangle. What it means is
	// the caller's — figure counts the rows and stops there.
	top.Plot().On(figure.Select, func(ev figure.Event) {
		status.SetText(fmt.Sprintf("%s: %d rows selected", ev.Hit.Series, len(ev.Rows)))
	})

	mode := widget.NewSelect([]string{"Select", "Zoom to band", "Pan"}, func(s string) {
		switch s {
		case "Select":
			top.SetDragMode(figure.DragSelects)
		case "Zoom to band":
			top.SetDragMode(figure.DragZooms)
		default:
			top.SetDragMode(figure.DragPans)
		}
	})
	mode.SetSelected("Select")

	show := widget.NewButton("Show every series", func() {
		if err := top.ShowAllLayers(); err != nil {
			status.SetText("showing: " + err.Error())
			return
		}
		_ = bottom.ShowAllLayers()
	})

	controls := container.NewHBox(widget.NewLabel("Drag:"), mode, show)
	charts := container.NewGridWithRows(2, top, bottom)
	return container.NewBorder(nil, container.NewVBox(status, controls), nil, nil, charts)
}

// sceneTab is one surface seen from two cameras. The scene is built once and
// its scales trained once; each view is only a camera on it, and a drag turns
// the one it started in.
func sceneTab() fyne.CanvasObject {
	sc := three.NewScene(three.XTitle("x"), three.YTitle("y"), three.ZTitle("response")).
		Z(scale.Linear(scale.Nice())).
		Add(three.Surface(saddle(), geom.X("x"), geom.Y("y"), geom.Z("z"),
			geom.Fill(palette.SkyBlue), geom.Label("response")))

	labels := []string{"three-quarter", "plan", "front", "side"}
	p := three.New(three.Size(900, 480), three.Title("Response surface"), three.Columns(2)).
		Scene(sc).
		Add(
			three.View{Camera: three.Home(), Label: labels[0]},
			three.View{Camera: three.LookAt(three.Elevation(1.45)), Label: labels[1]},
			three.View{Camera: three.LookAt(three.Azimuth(0), three.Elevation(0.02)), Label: labels[2]},
			three.View{Camera: three.LookAt(three.Azimuth(-math.Pi/2), three.Elevation(0.02)), Label: labels[3]},
		)

	c := orbit.New(p, orbit.Interactive(true), orbit.Select(true))

	hint := "Drag a view to turn it, click a point to mark it in all four, double click to go home."
	status := widget.NewLabel(hint)
	c.OnCamera(func(i int, cam three.Camera) {
		status.SetText(fmt.Sprintf("%s   azimuth %.0f°   elevation %.0f°   zoom %.2f",
			labels[i], cam.Azimuth()*180/math.Pi, cam.Elevation()*180/math.Pi, cam.Zoom()))
	})
	// A projected mark has no x and y to read back, so a hover says which
	// view and which row — which is the whole answer a pointer has here.
	c.OnHover(func(h interact.Hit, found bool) {
		if !found {
			status.SetText(hint)
			return
		}
		status.SetText(fmt.Sprintf("%s   %s   row %d", labels[h.Panel], h.Series, h.Row))
	})
	// One click, four rings. A scene with four cameras is one chart looked at
	// four ways, so a point picked in the plan is the same point in the
	// three-quarter view and both profiles — which is the thing several views
	// are for and the thing a still picture of one cannot do.
	//
	// The handler runs with the chart held, so it must not call back into it —
	// c.ViewCount() here would be a deadlock against the click that is still
	// being delivered. Everything it needs is read before it is registered.
	views := len(labels)
	c.OnSelect(func(sel fynefigure.Selection) {
		if len(sel) == 0 {
			status.SetText(hint)
			return
		}
		status.SetText(fmt.Sprintf("row %d picked in the %s view, and marked in all %d",
			sel[0].Row, labels[sel[0].View], views))
	})

	home := widget.NewButton("Home", func() {
		if err := c.Home(); err != nil {
			status.SetText("home: " + err.Error())
		}
	})
	clear := widget.NewButton("Clear selection", func() { c.SetSelection(nil) })
	return container.NewBorder(nil,
		container.NewVBox(status, container.NewHBox(home, clear)), nil, nil, c)
}

// saddle is a response with a ridge one way and a trough the other: the shape
// whose point is that a heatmap of it looks symmetric and it is not.
func saddle() figure.Source {
	const n = 28
	xs := make([]float64, 0, n*n)
	ys := make([]float64, 0, n*n)
	zs := make([]float64, 0, n*n)
	for j := range n {
		for i := range n {
			x := -3 + 6*float64(i)/(n-1)
			y := -3 + 6*float64(j)/(n-1)
			xs, ys = append(xs, x), append(ys, y)
			zs = append(zs, math.Sin(x)*math.Cos(y)*1.4+0.25*x)
		}
	}
	return figure.NewTable().Float64("x", xs).Float64("y", ys).Float64("z", zs)
}

// plotOf is two named series over one table, which is what gives the chart a
// legend to click.
func plotOf(src figure.Source, title string) *figure.Plot {
	p := figure.New(
		figure.Responsive(true),
		figure.Size(900, 240),
		figure.Title(title),
		figure.XTitle("t"),
		figure.YTitle("amplitude"),
	)
	p.Add(
		geom.Line(src, geom.X("t"), geom.Y("signal"),
			geom.Color(palette.SkyBlue), geom.Label("signal")),
		geom.Line(src, geom.X("t"), geom.Y("carrier"),
			geom.Color(palette.Vermilion), geom.Label("carrier")),
	)
	return p
}

// liveTab is a stream and a chart. It returns the chart and a function that
// starts repainting it, so that the caller can stop when the tab is hidden.
func liveTab() (fyne.CanvasObject, func() (stop func())) {
	const window = 600

	st := data.NewStream("t", "y").Window(window)
	seed(st, window)

	p := figure.New(
		figure.Responsive(true),
		figure.Size(900, 480),
		figure.Title("Live throughput"),
		figure.YTitle("rows/s"),
	)
	// A pinned Y axis is what makes a live chart readable: one that rescales
	// itself every frame turns every change into a redraw of everything, and
	// makes two frames impossible to compare by eye.
	p.Y(scale.Linear(scale.Domain(0, 120)))
	p.Add(
		geom.Line(st.Source(), geom.X("t"), geom.Y("y"),
			geom.Color(palette.SkyBlue), geom.Label("throughput")),
		geom.HLine(100, geom.Label("capacity"), geom.Dash(6, 4)),
	)

	// The x axis follows the window: a sliding stream leaves its oldest sample
	// behind on every frame, and an axis that remembered it would pin the left
	// edge to the first sample and never move. That is chart.Follow, and it is
	// the default — as is ignoring a drag here, because the rows behind the
	// tip have been dropped and there is nothing back there to look at. Add
	// chart.FollowPause(true) to let a drag take the chart off the data
	// instead, until a double click hands it back.
	c := chart.New(p, chart.Interactive(true))
	c.Stream(st)

	// The producer appends and never reads. The chart freezes a snapshot
	// between frames, so it never sees the stream half-written.
	go func() {
		t := time.NewTicker(40 * time.Millisecond)
		defer t.Stop()
		last := float64(window)
		for range t.C {
			last++
			if err := st.Append(last, throughput(last)); err != nil {
				log.Println("append:", err)
				return
			}
		}
	}()

	return c, func() func() { return c.Animate(50 * time.Millisecond) }
}

func seed(st *data.Stream, n int) {
	for i := range n {
		t := float64(i)
		if err := st.Append(t, throughput(t)); err != nil {
			log.Fatalln("seeding the stream:", err)
		}
	}
}

func throughput(t float64) float64 {
	return 70 + 25*math.Sin(t/50) + 8*rand.Float64()
}

func signal(n int) figure.Source {
	x := make([]float64, n)
	y := make([]float64, n)
	carrier := make([]float64, n)
	for i := range n {
		t := float64(i) / 40
		x[i] = t
		y[i] = math.Sin(t) + 0.35*math.Sin(7.3*t) + 0.12*math.Sin(31*t)
		carrier[i] = 0.8 * math.Cos(t/1.7)
	}
	return figure.Float64Columns(map[string][]float64{"t": x, "signal": y, "carrier": carrier})
}
