//go:build tierparity

// This file is behind a tag because it gives the GPU back halfway through, and
// the accelerator is registered process-wide: every benchmark that ran after it
// would measure the CPU rasterizer while saying GPU. Run it on its own:
//
//	go test -tags tierparity -run Parity ./gpu

package gpu_test

import (
	"bytes"
	"image"
	"image/png"
	"math"
	"testing"

	gputier "github.com/timzifer/fyne-refract/gpu"
	"github.com/timzifer/refract"
	ggbackend "github.com/timzifer/refract/backend/gg"
	"github.com/timzifer/refract/geom"
)

// A chart drawn on the GPU should be the chart drawn on the CPU. refract's own
// position is that the tier changes which coverage filler rasterizes the marks
// and nothing else — this is what that claim costs in pixels, measured rather
// than assumed, because a chart that looked different on a machine with a GPU
// would be a chart nobody could compare with its own exported PNG.
func TestParityBetweenTheTiers(t *testing.T) {
	if !gputier.Enabled() {
		t.Skip("no GPU tier on this machine; there is nothing to compare")
	}
	onGPU := render(t)

	gputier.Close()
	if gputier.Enabled() {
		t.Fatal("the tier is still enabled after Close")
	}
	onCPU := render(t)

	if onGPU.Bounds() != onCPU.Bounds() {
		t.Fatalf("the two tiers drew %v and %v", onGPU.Bounds(), onCPU.Bounds())
	}
	differing, worst, mean := compare(onGPU, onCPU)
	total := onGPU.Bounds().Dx() * onGPU.Bounds().Dy()
	t.Logf("%d of %d pixels differ (%.1f%%), worst channel by %d, mean %.2f",
		differing, total, 100*float64(differing)/float64(total), worst, mean)

	// The two tiers do not share a coverage filler, so they will not agree
	// pixel for pixel and asking them to would be asking the wrong question.
	// What has to hold is that they drew the same chart: the difference lives
	// on the edges of what was drawn — the line, the grid, the glyphs — and
	// nowhere else. A filled area in the wrong place, a missing mark or a
	// dropped path shows up as a large share of pixels and a high mean; an
	// antialiased edge shows up as a small share and a low one.
	if frac := float64(differing) / float64(total); frac > 0.12 {
		t.Errorf("%.1f%% of pixels differ, want the edges only", 100*frac)
	}
	if mean > 8 {
		t.Errorf("the mean channel difference is %.2f, want the edges only", mean)
	}
	if worst == 255 && differing > total/2 {
		t.Error("one tier drew something the other did not draw at all")
	}
}

func render(t *testing.T) image.Image {
	t.Helper()
	const n = 400
	x := make([]float64, n)
	y := make([]float64, n)
	for i := range n {
		v := float64(i) / 20
		x[i], y[i] = v, math.Sin(v)
	}
	src := refract.Float64Columns(map[string][]float64{"t": x, "y": y})

	p := refract.New(refract.Size(400, 250), refract.Title("Signal"))
	p.Add(geom.Line(src, geom.X("t"), geom.Y("y")))

	var buf bytes.Buffer
	if err := p.Render(ggbackend.Writer(&buf, ggbackend.FormatPNG)); err != nil {
		t.Fatalf("rendering: %v", err)
	}
	img, err := png.Decode(&buf)
	if err != nil {
		t.Fatalf("decoding: %v", err)
	}
	return img
}

func compare(a, b image.Image) (differing, worst int, mean float64) {
	r := a.Bounds()
	var total int64
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			ar, ag, ab, aa := a.At(x, y).RGBA()
			br, bg, bb, ba := b.At(x, y).RGBA()
			d := max4(diff(ar, br), diff(ag, bg), diff(ab, bb), diff(aa, ba))
			total += int64(d)
			if d > 0 {
				differing++
				worst = max(worst, d)
			}
		}
	}
	n := int64(r.Dx()) * int64(r.Dy())
	return differing, worst, float64(total) / float64(n)
}

func diff(a, b uint32) int {
	d := int(a>>8) - int(b>>8)
	if d < 0 {
		return -d
	}
	return d
}

func max4(a, b, c, d int) int { return max(max(a, b), max(c, d)) }
