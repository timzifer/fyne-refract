package gpu_test

import (
	"math"
	"testing"

	gputier "github.com/timzifer/fyne-refract/gpu"
	"github.com/timzifer/refract"
	ggbackend "github.com/timzifer/refract/backend/gg"
	"github.com/timzifer/refract/data"
	"github.com/timzifer/refract/geom"
	"github.com/timzifer/refract/scale"
)

// Whether the tier took depends on the machine: a runner with no Vulkan, Metal
// or DX12 says no and rendering falls back to the CPU, which is the documented
// behaviour and not a failure. So this reports rather than asserts.
//
// What it deliberately does not do is call Close. The accelerator is
// registered in gg, process-wide, and giving it back would take it away from
// every test and benchmark that runs after — which would then quietly measure
// the CPU and be read as the GPU. Close is exercised by the parity test, which
// is behind a build tag for that reason.
func TestTheTierSaysWhetherItTook(t *testing.T) {
	t.Logf("GPU tier enabled: %v", gputier.Enabled())
}

// The same frames the package next door benchmarks, so that the two tables can
// be read side by side. They go through backend/gg's surface directly rather
// than through the widget's target, which is the same surface with a Fyne
// object attached: what the tier changes is the rasterizer underneath both.
//
//	go test -run='^$' -bench=Frame -benchtime=30x ./gpu
func BenchmarkFrame(b *testing.B) {
	b.Logf("GPU tier enabled: %v", gputier.Enabled())
	for _, damage := range []bool{false, true} {
		name := "wholeFrames"
		if damage {
			name = "partialFrames"
		}
		b.Run(name, func(b *testing.B) {
			b.Run("streamSliding", func(b *testing.B) { benchStream(b, 900, 600, 600, damage) })
			b.Run("panStatic", func(b *testing.B) { benchPan(b, 900, damage) })
		})
	}
}

func benchStream(b *testing.B, width, window, seed int, damage bool) {
	st := data.NewStream("t", "y").Window(window)
	for i := range seed {
		if err := st.Append(float64(i), 70+25*math.Sin(float64(i)/50)); err != nil {
			b.Fatal(err)
		}
	}
	p := refract.New(refract.Size(width, width*8/15))
	p.X(scale.Linear(scale.Domain(0, float64(window))))
	p.Y(scale.Linear(scale.Domain(0, 120)))
	p.Add(geom.Line(st.Source(), geom.X("t"), geom.Y("y")))

	live := open(b, p, width, damage)
	defer live.Close()
	st.Snapshot()
	if err := live.Draw(); err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for i := 0; b.Loop(); i++ {
		t := float64(seed + i)
		if err := st.Append(t, 70+25*math.Sin(t/50)); err != nil {
			b.Fatal(err)
		}
		st.Snapshot()
		if err := live.Draw(); err != nil {
			b.Fatal(err)
		}
	}
}

func benchPan(b *testing.B, width int, damage bool) {
	n := 4000
	x := make([]float64, n)
	y := make([]float64, n)
	for i := range n {
		t := float64(i) / 40
		x[i] = t
		y[i] = math.Sin(t) + 0.35*math.Sin(7.3*t) + 0.12*math.Sin(31*t)
	}
	src := refract.Float64Columns(map[string][]float64{"t": x, "signal": y})

	p := refract.New(refract.Size(width, width*8/15), refract.Title("Signal"))
	p.Add(geom.Line(src, geom.X("t"), geom.Y("signal")))

	live := open(b, p, width, damage)
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

func open(b *testing.B, p *refract.Plot, width int, damage bool) *refract.Live {
	b.Helper()
	var target refract.Target = ggbackend.NewSurface()
	if !damage {
		target = whole{ggbackend.NewSurface()}
	}
	live, err := p.Live(target)
	if err != nil {
		b.Fatal(err)
	}
	if err := live.Resize(width, width*8/15); err != nil {
		b.Fatal(err)
	}
	return live
}
