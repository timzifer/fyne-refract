package chart_test

import (
	"image/color"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	fynetheme "fyne.io/fyne/v2/theme"
	"github.com/timzifer/fyne_figure/chart"
)

func TestAChartFollowsTheApplicationsColours(t *testing.T) {
	app := test.NewTempApp(t)
	app.Settings().SetTheme(paper{color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff}})

	c, _ := laidOut(t, chart.New(plot(), chart.ThemeFont(false)))
	light := topLeft(t, c)

	app.Settings().SetTheme(paper{color.NRGBA{R: 0x10, G: 0x12, B: 0x16, A: 0xff}})
	c.Refresh()
	if err := c.Err(); err != nil {
		t.Fatalf("the frame after the theme changed failed: %v", err)
	}
	dark := topLeft(t, c)

	if dark == light {
		t.Fatalf("the chart's page is %v under a white theme and a black one", light)
	}
	if luma(dark) >= luma(light) {
		t.Errorf("the dark theme drew the lighter page: %v against %v", dark, light)
	}
}

func TestAChartCanIgnoreTheTheme(t *testing.T) {
	app := test.NewTempApp(t)
	app.Settings().SetTheme(paper{color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff}})

	c, _ := laidOut(t, chart.New(plot(), chart.ThemeFont(false), chart.FollowTheme(false)))
	before := topLeft(t, c)

	app.Settings().SetTheme(paper{color.NRGBA{R: 0x10, G: 0x12, B: 0x16, A: 0xff}})
	c.Refresh()
	if got := topLeft(t, c); got != before {
		t.Errorf("a chart told not to follow the theme changed its page from %v to %v", before, got)
	}
}

func laidOut(t *testing.T, c *chart.Chart) (*chart.Chart, fyne.Window) {
	t.Helper()
	win := test.NewTempWindow(t, c)
	win.Resize(fyne.NewSize(500, 300))
	c.Resize(fyne.NewSize(500, 300))
	if err := c.Err(); err != nil {
		t.Fatalf("laying the chart out: %v", err)
	}
	return c, win
}

// topLeft is the colour of the chart's first pixel, which is outside every
// panel and so is the page and nothing else.
func topLeft(t *testing.T, c *chart.Chart) [4]uint32 {
	t.Helper()
	img := c.Target().Image()
	if img == nil {
		t.Fatal("the chart has no pixels")
	}
	r, g, b, a := img.At(img.Bounds().Min.X, img.Bounds().Min.Y).RGBA()
	return [4]uint32{r, g, b, a}
}

func luma(c [4]uint32) float64 {
	return 0.2126*float64(c[0]) + 0.7152*float64(c[1]) + 0.0722*float64(c[2])
}

// paper is Fyne's default theme with the background replaced, which is the one
// colour a chart reads to decide whether it is a light chart or a dark one.
type paper struct{ bg color.Color }

func (p paper) Color(n fyne.ThemeColorName, v fyne.ThemeVariant) color.Color {
	if n == fynetheme.ColorNameBackground {
		return p.bg
	}
	return fynetheme.DefaultTheme().Color(n, v)
}

func (paper) Font(s fyne.TextStyle) fyne.Resource { return fynetheme.DefaultTheme().Font(s) }
func (paper) Icon(n fyne.ThemeIconName) fyne.Resource {
	return fynetheme.DefaultTheme().Icon(n)
}
func (paper) Size(n fyne.ThemeSizeName) float32 { return fynetheme.DefaultTheme().Size(n) }

func TestAChangeOfTypefaceKeepsTheObjectTheWidgetIsShowing(t *testing.T) {
	app := test.NewTempApp(t)
	app.Settings().SetTheme(paper{color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff}})

	c, _ := laidOut(t, chart.New(plot()))
	shown := test.WidgetRenderer(c).Objects()[0]

	app.Settings().SetTheme(monospaced{paper{color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff}}})
	c.Refresh()

	if err := c.Err(); err != nil {
		t.Fatalf("the frame after the typeface changed failed: %v", err)
	}
	if got := test.WidgetRenderer(c).Objects()[0]; got != shown {
		t.Error("a change of typeface replaced the object the widget is showing")
	}
	if c.Target().Object() != shown {
		t.Error("the target draws into an object the widget is not showing")
	}
	if c.Live() == nil {
		t.Fatal("the chart was not opened again after the typeface changed")
	}
	if w, h := c.Live().Size(); w != 500 || h != 300 {
		t.Errorf("after the typeface changed the chart is %dx%d, want 500x300", w, h)
	}
	if c.Target().Frames() == 0 {
		t.Error("nothing was painted after the typeface changed")
	}
}

// monospaced is a theme whose regular face is the monospace one, which is a
// different font resource and so a different typeface to draw labels in.
type monospaced struct{ fyne.Theme }

func (m monospaced) Font(s fyne.TextStyle) fyne.Resource {
	s.Monospace = true
	return m.Theme.Font(s)
}
