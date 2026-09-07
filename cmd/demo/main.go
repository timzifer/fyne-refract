// Command demo shows refract charts in a Fyne window.
//
//	go run ./cmd/demo
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
	"github.com/timzifer/fyne-refract/chart"
	"github.com/timzifer/refract"
	ggbackend "github.com/timzifer/refract/backend/gg"
	"github.com/timzifer/refract/data"
	"github.com/timzifer/refract/geom"
	"github.com/timzifer/refract/palette"
	"github.com/timzifer/refract/scale"
)

func main() {
	a := app.New()
	w := a.NewWindow("refract — Fyne")
	w.Resize(fyne.NewSize(960, 620))

	live, animate := liveTab()
	liveItem := container.NewTabItem("Live", live)

	tabs := container.NewAppTabs(
		container.NewTabItem("Signal", signalTab()),
		liveItem,
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
	p := refract.New(
		refract.Responsive(true),
		refract.Size(900, 480),
		refract.Title("Signal"),
		refract.XTitle("t"),
		refract.YTitle("amplitude"),
	)
	p.Add(
		geom.Line(signal(4000), geom.X("t"), geom.Y("signal"),
			geom.Color(palette.SkyBlue), geom.Label("signal")),
	)

	// chart.Detail(0.5) here would draw the chart at half resolution while it
	// is dragged and sharpen it afterwards, which is worth about three times
	// the frame rate on the CPU rasterizer. It is left off so that the demo
	// shows what the widget does by default.
	c := chart.New(p, chart.TrackRows(true))

	status := widget.NewLabel("Hover the chart.")
	p.On(refract.Hover, func(ev refract.Event) {
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
		path := filepath.Join(os.TempDir(), "refract-demo.png")
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

// liveTab is a stream and a chart. It returns the chart and a function that
// starts repainting it, so that the caller can stop when the tab is hidden.
func liveTab() (fyne.CanvasObject, func() (stop func())) {
	const window = 600

	st := data.NewStream("t", "y").Window(window)
	seed(st, window)

	p := refract.New(
		refract.Responsive(true),
		refract.Size(900, 480),
		refract.Title("Live throughput"),
		refract.YTitle("rows/s"),
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
	c := chart.New(p)
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

func signal(n int) refract.Source {
	x := make([]float64, n)
	y := make([]float64, n)
	for i := range n {
		t := float64(i) / 40
		x[i] = t
		y[i] = math.Sin(t) + 0.35*math.Sin(7.3*t) + 0.12*math.Sin(31*t)
	}
	return refract.Float64Columns(map[string][]float64{"t": x, "signal": y})
}
