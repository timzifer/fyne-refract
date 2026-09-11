// Package look reads what a Fyne theme asks of a chart: the page it sits on,
// the size of the text around it and the typeface that text is set in.
//
// Both widgets follow it — package chart for a flat plot and package orbit for
// a projected one — so what "follows the application's colours" means is
// written down once.
package look

import (
	"image/color"

	"fyne.io/fyne/v2"
	fynetheme "fyne.io/fyne/v2/theme"
	"github.com/timzifer/figure/ir"
	figuretheme "github.com/timzifer/figure/theme"
)

// State is what a chart was last built for. Comparing two of them is how a
// settings change that touched neither the palette nor the typeface is
// recognised as nothing to do — Fyne fires a settings change for a scale, a
// primary colour or an animation preference too.
type State struct {
	Background color.RGBA
	Size       float32
	Font       string
}

// Read reports what th asks for under the application's current variant. It is
// the zero State outside an application or without a theme.
func Read(th fyne.Theme) State {
	app := fyne.CurrentApp()
	if app == nil || th == nil {
		return State{}
	}
	variant := app.Settings().ThemeVariant()

	st := State{
		Background: RGBA(th.Color(fynetheme.ColorNameBackground, variant)),
		Size:       th.Size(fynetheme.SizeNameText),
	}
	if res := th.Font(fyne.TextStyle{}); res != nil {
		st.Font = res.Name()
	}
	return st
}

// Theme is figure's own light or dark theme, in the page colour and at the
// text size s names.
//
// Which of the two is decided by how dark the background is rather than by
// Fyne's light/dark preference, because a Fyne theme is not obliged to be
// either: a custom one is whatever colours it names, and its background is the
// honest answer to "is this a dark chart or a light one". The page is Fyne's,
// so the chart sits in the widget rather than on a rectangle of its own.
func (s State) Theme() figuretheme.Theme {
	base := figuretheme.Light
	if Dark(s.Background) {
		base = figuretheme.Dark
	}
	opts := []figuretheme.Option{
		figuretheme.Background(ir.RGBA(s.Background.R, s.Background.G, s.Background.B, s.Background.A)),
	}
	if s.Size > 0 {
		opts = append(opts, figuretheme.FontSize(float64(s.Size)))
	}
	return base.With(opts...)
}

// Dark reports whether a background wants a dark chart. The weights are the
// sRGB luma ones and the threshold is the middle.
func Dark(c color.RGBA) bool {
	if c.A == 0 {
		return false
	}
	luma := 0.2126*float64(c.R) + 0.7152*float64(c.G) + 0.0722*float64(c.B)
	return luma < 128
}

// Fonts reads the application's typeface, for the rasterizer to draw labels
// with. Bold and italic are optional: a theme with no italic face gets the
// regular one, which is what the rasterizer does with a nil.
func Fonts(th fyne.Theme) (regular, bold, italic []byte, ok bool) {
	if fyne.CurrentApp() == nil || th == nil {
		return nil, nil, nil, false
	}
	regular = fontBytes(th, fyne.TextStyle{})
	if len(regular) == 0 {
		return nil, nil, nil, false
	}
	return regular, fontBytes(th, fyne.TextStyle{Bold: true}), fontBytes(th, fyne.TextStyle{Italic: true}), true
}

func fontBytes(th fyne.Theme, style fyne.TextStyle) []byte {
	res := th.Font(style)
	if res == nil {
		return nil
	}
	return res.Content()
}

// RGBA flattens a theme colour into the eight-bit non-premultiplied channels
// figure's palette speaks in.
func RGBA(c color.Color) color.RGBA {
	if c == nil {
		return color.RGBA{}
	}
	r, g, b, a := c.RGBA()
	if a == 0 {
		return color.RGBA{}
	}
	return color.RGBA{
		R: uint8(r * 0xffff / a >> 8),
		G: uint8(g * 0xffff / a >> 8),
		B: uint8(b * 0xffff / a >> 8),
		A: uint8(a >> 8),
	}
}
