package fynefigure_test

import (
	"math"
	"strconv"
	"testing"

	"github.com/timzifer/figure"
	"github.com/timzifer/figure/data"
	"github.com/timzifer/figure/geom"
	"github.com/timzifer/figure/scale"
	fynefigure "github.com/timzifer/fyne-figure"
)

// What a frame costs, and whether repainting only the part of it that changed
// is worth what it costs. The answer is in [fynefigure.DamageBudget]; these
// are the measurements behind it, and what to re-run before changing its
// default.
//
//	go test -run='^$' -bench=Frame -benchtime=30x .
//
// What a frame costs at a fraction of the device resolution, which is the
// question behind drawing a coarser chart while a reader is dragging: the
// rasterizer's work is per pixel, and half the linear scale is a quarter of
// the pixels.
//
//	go test -run='^$' -bench=Resolution -benchtime=25x .
func BenchmarkResolution(b *testing.B) {
	for _, dpr := range []float64{1, 0.75, 0.5, 0.35} {
		b.Run("dpr="+strconv.FormatFloat(dpr, 'g', -1, 64), func(b *testing.B) {
			b.Run("panStatic", func(b *testing.B) { benchPanAt(b, dpr) })
			b.Run("streamSliding", func(b *testing.B) { benchStreamAt(b, dpr) })
		})
	}
}

func benchPanAt(b *testing.B, dpr float64) {
	p := figure.New(figure.Size(900, 480), figure.Title("Signal"))
	p.Add(geom.Line(signal(4000), geom.X("t"), geom.Y("signal")))

	live := open(b, p, 0)
	defer live.Close()
	if err := live.Rescale(dpr); err != nil {
		b.Fatal(err)
	}
	if err := live.Draw(); err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for i := 0; b.Loop(); i++ {
		if err := live.PanBy(float64(i%3)-1, 0); err != nil {
			b.Fatal(err)
		}
	}
}

func benchStreamAt(b *testing.B, dpr float64) {
	st := data.NewStream("t", "y").Window(600)
	for i := range 600 {
		if err := st.Append(float64(i), throughput(float64(i))); err != nil {
			b.Fatal(err)
		}
	}
	p := figure.New(figure.Size(900, 480))
	p.X(scale.Linear(scale.Domain(0, 600)))
	p.Y(scale.Linear(scale.Domain(0, 120)))
	p.Add(geom.Line(st.Source(), geom.X("t"), geom.Y("y")))

	live := open(b, p, 0)
	defer live.Close()
	if err := live.Rescale(dpr); err != nil {
		b.Fatal(err)
	}
	st.Snapshot()
	if err := live.Draw(); err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for i := 0; b.Loop(); i++ {
		t := float64(600 + i)
		if err := st.Append(t, throughput(t)); err != nil {
			b.Fatal(err)
		}
		st.Snapshot()
		if err := live.Draw(); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkFrame(b *testing.B) {
	for _, budget := range []float32{0, 0.3, 1} {
		b.Run("budget="+strconv.FormatFloat(float64(budget), 'g', -1, 32), func(b *testing.B) {
			b.Run("streamSliding", func(b *testing.B) { benchStream(b, budget, 600, 600) })
			b.Run("streamTip", func(b *testing.B) { benchStream(b, budget, 20000, 2000) })
			b.Run("panStatic", func(b *testing.B) { benchPan(b, budget) })
		})
	}
}

// benchStream is a live chart: one sample appended per frame. With a window
// the size of the seed the whole line slides left every frame; with a window
// far larger, only the right-hand tip is new.
func benchStream(b *testing.B, budget float32, window, seed int) {
	st := data.NewStream("t", "y").Window(window)
	for i := range seed {
		if err := st.Append(float64(i), throughput(float64(i))); err != nil {
			b.Fatal(err)
		}
	}
	p := figure.New(figure.Size(900, 480))
	p.X(scale.Linear(scale.Domain(0, float64(window))))
	p.Y(scale.Linear(scale.Domain(0, 120)))
	p.Add(geom.Line(st.Source(), geom.X("t"), geom.Y("y")))

	live := open(b, p, budget)
	defer live.Close()
	st.Snapshot()
	if err := live.Draw(); err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for i := 0; b.Loop(); i++ {
		t := float64(seed + i)
		if err := st.Append(t, throughput(t)); err != nil {
			b.Fatal(err)
		}
		st.Snapshot()
		if err := live.Draw(); err != nil {
			b.Fatal(err)
		}
	}
}

// benchPan is a reader dragging a static chart, which is the case a partial
// repaint is supposed to be for: the furniture does not move, only the marks.
func benchPan(b *testing.B, budget float32) {
	p := figure.New(figure.Size(900, 480), figure.Title("Signal"))
	p.Add(geom.Line(signal(4000), geom.X("t"), geom.Y("signal")))

	live := open(b, p, budget)
	defer live.Close()
	if err := live.Draw(); err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for i := 0; b.Loop(); i++ {
		if err := live.PanBy(float64(i%3)-1, 0); err != nil {
			b.Fatal(err)
		}
	}
}

func open(b *testing.B, p *figure.Plot, budget float32) *figure.Live {
	b.Helper()
	live, err := p.Live(fynefigure.New(fynefigure.DamageBudget(budget)))
	if err != nil {
		b.Fatal(err)
	}
	if err := live.Resize(900, 480); err != nil {
		b.Fatal(err)
	}
	return live
}

func throughput(t float64) float64 { return 70 + 25*math.Sin(t/50) }

func signal(n int) figure.Source {
	x := make([]float64, n)
	y := make([]float64, n)
	for i := range n {
		t := float64(i) / 40
		x[i] = t
		y[i] = math.Sin(t) + 0.35*math.Sin(7.3*t) + 0.12*math.Sin(31*t)
	}
	return figure.Float64Columns(map[string][]float64{"t": x, "signal": y})
}
