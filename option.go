package fynerefract

import (
	"fyne.io/fyne/v2/canvas"
	ggbackend "github.com/timzifer/refract/backend/gg"
)

// Option configures a [Target].
type Option func(*config)

type config struct {
	gg     []ggbackend.Option
	scale  canvas.ImageScale
	budget float32
}

// Font replaces the rasterizer's embedded Go fonts with supplied TrueType or
// OpenType files. Pass bold or italic as nil to reuse regular for that style.
//
// It is how a chart is drawn in the application's own typeface: package
// fynerefract/chart reads the faces off the Fyne theme and passes them here.
// Without it a chart uses the same fonts every other refract raster does,
// which is what makes its pixels comparable with an exported PNG.
func Font(regular, bold, italic []byte) Option {
	return func(c *config) { c.gg = append(c.gg, ggbackend.WithFont(regular, bold, italic)) }
}

// ScaleMode sets how Fyne resamples the chart if it ever has to.
//
// It normally does not: the raster is generated at exactly the pixel size the
// painter asks for. The exception is the one frame after a display's device
// pixel ratio changes, where the previous frame is stretched while the next is
// rasterized at the new ratio. The default is [canvas.ImageScaleSmooth].
func ScaleMode(m canvas.ImageScale) Option {
	return func(c *config) { c.scale = m }
}

// DamageBudget turns partial repaints back on, for frames whose damaged region
// covers no more than f of the surface. Zero, the default, repaints the whole
// frame every time.
//
// refract works out where a frame changed and offers the rasterizer the chance
// to repaint only that. It sounds like a saving and here it is not: the
// rasterizer clears the damaged box and clips every drawing call to it, and its
// clip is a mask it rasterizes across the surface and then samples per pixel —
// on top of the clip a panel already puts there. Measured on a 900x480 chart,
// per frame:
//
//	                          whole frame   partial
//	a stream sliding left        29 ms       180 ms
//	a stream growing at its
//	  right-hand tip             30 ms       170 ms
//	panning a static chart       50 ms        50 ms
//
// Six times slower where it does anything, and a wash where it does not. So
// the default is off, and this is here for a chart whose changes really are
// confined to a corner of it, and for the day the rasterizer's clip gets
// cheaper. There is a benchmark; measure before turning it on.
func DamageBudget(f float32) Option {
	return func(c *config) {
		if f < 0 {
			f = 0
		}
		c.budget = f
	}
}

func build(opts []Option) config {
	c := config{scale: canvas.ImageScaleSmooth}
	for _, o := range opts {
		o(&c)
	}
	return c
}
