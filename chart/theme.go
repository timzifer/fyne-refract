package chart

import (
	"image/color"

	"fyne.io/fyne/v2"
	fynetheme "fyne.io/fyne/v2/theme"
	"github.com/timzifer/refract"
	"github.com/timzifer/refract/ir"
	refracttheme "github.com/timzifer/refract/theme"
)

// themeState is what a chart was last built for. Comparing two of them is how
// a settings change that touched neither the palette nor the typeface is
// recognised as nothing to do — Fyne fires a settings change for a scale, a
// primary colour or an animation preference too.
type themeState struct {
	background color.RGBA
	size       float32
	font       string
}

// syncTheme follows Fyne's own colours and typeface, and rebuilds the chart
// when either moved. It runs on every Refresh, which is where Fyne has already
// told the widget its theme may have changed.
func (c *Chart) syncTheme() {
	if !c.cfg.theme && !c.cfg.font {
		return
	}
	now := c.themeStateNow()
	if now == c.themed {
		return
	}
	fontChanged := c.cfg.font && now.font != c.themed.font
	c.themed = now

	// The typeface is fixed when a rasterizer is made, so a new one means a new
	// rasterizer — and, because a Live holds the backend it was handed, a new
	// chart too. The target survives both, because the widget is holding its
	// canvas object. A change of colour is only a rebuild.
	if fontChanged {
		c.refont()
		return
	}
	if c.live != nil && c.cfg.theme {
		c.applyTheme()
		if err := c.live.Rebuild(); err != nil {
			c.renderr = err
		}
	}
}

// refont rebuilds the rasterizer in the typeface the theme now asks for, and
// opens the chart again into it.
func (c *Chart) refont() {
	if c.target == nil {
		return
	}
	size := fyne.NewSize(float32(c.w), float32(c.h))
	if c.live != nil {
		// Closing a Live closes the target it was given; the surface is
		// replaced immediately below, and the canvas object is not.
		_ = c.live.Close()
		c.live, c.in = nil, nil
	}

	regular, bold, italic, ok := c.themeFonts()
	if !ok {
		regular, bold, italic = nil, nil, nil
	}
	if err := c.target.SetFont(regular, bold, italic); err != nil {
		c.renderr = err
	}
	// The tooltip is drawn by a rasterizer of its own, in the same typeface:
	// a box in a different face from the axis beside it would read as a bug.
	c.tip.setFont(regular, bold, italic)
	c.w, c.h, c.dpr = 0, 0, 0
	c.resize(size)
}

// applyTheme puts refract's own light or dark theme on the plot, in the page
// colour and at the text size Fyne asks for.
//
// Which of the two is decided by how dark the application's background is
// rather than by Fyne's light/dark preference, because a Fyne theme is not
// obliged to be either: a custom one is whatever colours it names, and its
// background is the honest answer to "is this a dark chart or a light one".
//
// It is refract.Theme applied to the plot directly rather than at
// construction: a Plot Option is an ordinary function, and a chart whose
// surroundings changed colour has not become a different chart.
func (c *Chart) applyTheme() {
	if !c.cfg.theme {
		return
	}
	st := c.themeStateNow()

	base := refracttheme.Light
	if dark(st.background) {
		base = refracttheme.Dark
	}
	// The page is Fyne's, so the chart sits in the widget rather than on a
	// rectangle of its own.
	opts := []refracttheme.Option{
		refracttheme.Background(ir.RGBA(st.background.R, st.background.G, st.background.B, st.background.A)),
	}
	if st.size > 0 {
		opts = append(opts, refracttheme.FontSize(float64(st.size)))
	}
	refract.Theme(base.With(opts...))(c.plot)
}

// dark reports whether a background wants a dark chart. The weights are the
// sRGB luma ones and the threshold is the middle.
func dark(c color.RGBA) bool {
	if c.A == 0 {
		return false
	}
	luma := 0.2126*float64(c.R) + 0.7152*float64(c.G) + 0.0722*float64(c.B)
	return luma < 128
}

// themeStateNow reads what Fyne currently asks for.
func (c *Chart) themeStateNow() themeState {
	app := fyne.CurrentApp()
	if app == nil {
		return themeState{}
	}
	th := c.Theme()
	if th == nil {
		return themeState{}
	}
	variant := app.Settings().ThemeVariant()

	st := themeState{
		background: rgba(th.Color(fynetheme.ColorNameBackground, variant)),
		size:       th.Size(fynetheme.SizeNameText),
	}
	if res := th.Font(fyne.TextStyle{}); res != nil {
		st.font = res.Name()
	}
	return st
}

// themeFonts reads the application's typeface, for the rasterizer to draw
// labels with. Bold and italic are optional: a theme with no italic face gets
// the regular one, which is what the rasterizer does with a nil.
func (c *Chart) themeFonts() (regular, bold, italic []byte, ok bool) {
	if fyne.CurrentApp() == nil {
		return nil, nil, nil, false
	}
	th := c.Theme()
	if th == nil {
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

// rgba flattens a theme colour into the eight-bit non-premultiplied channels
// refract's palette speaks in.
func rgba(c color.Color) color.RGBA {
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
