package chart

import (
	"fyne.io/fyne/v2"
	"github.com/timzifer/figure"
	"github.com/timzifer/fyne-figure/internal/look"
)

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
	fontChanged := c.cfg.font && now.Font != c.themed.Font
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

// applyTheme puts figure's own light or dark theme on the plot, in the page
// colour and at the text size Fyne asks for — see [look.State.Theme].
//
// It is figure.Theme applied to the plot directly rather than at
// construction: a Plot Option is an ordinary function, and a chart whose
// surroundings changed colour has not become a different chart.
func (c *Chart) applyTheme() {
	if !c.cfg.theme {
		return
	}
	figure.Theme(c.themeStateNow().Theme())(c.plot)
}

// themeStateNow reads what Fyne currently asks for.
func (c *Chart) themeStateNow() look.State {
	if fyne.CurrentApp() == nil {
		return look.State{}
	}
	return look.Read(c.Theme())
}

// themeFonts reads the application's typeface, for the rasterizer to draw
// labels with.
func (c *Chart) themeFonts() (regular, bold, italic []byte, ok bool) {
	if fyne.CurrentApp() == nil {
		return nil, nil, nil, false
	}
	return look.Fonts(c.Theme())
}
