package orbit

import (
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver/desktop"
	"github.com/timzifer/figure/three"
)

// Option configures a [Chart].
type Option func(*config)

type config struct {
	interactive bool

	min       fyne.Size
	perPixel  float64
	wheel     float64
	together  bool
	theme     bool
	trackRows bool
	interval  time.Duration
	cursor    desktop.Cursor

	selects     bool
	multiSelect bool
	ring        three.Highlight

	detail float32
}

func defaults() config {
	return config{
		min:      fyne.NewSize(240, 160),
		perPixel: DefaultPerPixel,
		wheel:    DefaultWheelScale,
		theme:    true,
		cursor:   desktop.DefaultCursor,
	}
}

// DefaultPerPixel is how far a pixel of drag turns the camera, in radians.
// A drag across a chart four hundred pixels wide is a little under a
// half-turn, which is enough to see round a surface without the scene
// spinning away from a reader who only meant to nudge it.
const DefaultPerPixel = 0.008

// DefaultWheelScale converts one unit of Fyne's scroll delta into the wheel
// units a dolly is measured in, where a thousand is a factor of e. It is the
// same number [chart.DefaultWheelScale] is, so a notch that zooms a flat chart
// by some amount dollies a projected one by the same.
const DefaultWheelScale = 4

// Interactive lets a reader turn the scene: a drag, the wheel, a double click
// and a hover. It is off by default.
//
// Off, the chart is a picture. It takes no pointer events at all, so it scrolls
// with a scroll container around it and leaves every gesture to whatever is
// behind it. The cameras can still be moved from code — [Chart.SetCamera] and
// [Chart.Home] work on a chart nobody can touch.
//
// [Chart.SetInteractive] changes it on a chart already on screen.
func Interactive(on bool) Option { return func(c *config) { c.interactive = on } }

// MinSize sets the smallest size the widget asks its layout for. The default
// is 240x160.
func MinSize(w, h float32) Option {
	return func(c *config) { c.min = fyne.NewSize(w, h) }
}

// PerPixel sets how far a pixel of drag turns the camera, in radians. The
// default is [DefaultPerPixel], and a drag takes hold of the scene: the side
// facing the reader follows the pointer. A negative value moves the camera
// with the pointer instead, so the scene turns against it.
func PerPixel(rad float64) Option {
	return func(c *config) {
		if rad != 0 {
			c.perPixel = rad
		}
	}
}

// WheelScale sets how much of a dolly one unit of Fyne's scroll delta is
// worth. The default is [DefaultWheelScale]; a negative value inverts the
// direction.
func WheelScale(f float64) Option {
	return func(c *config) {
		if f != 0 {
			c.wheel = f
		}
	}
}

// Together makes a drag or a wheel turn every view by the same amount, rather
// than only the view it started in. It is off by default. See the package
// documentation for which to want.
func Together(on bool) Option { return func(c *config) { c.together = on } }

// FollowTheme sets whether the chart takes its colours and text size from
// Fyne's theme. It is on by default; turn it off to keep the theme the plot
// was built with.
func FollowTheme(on bool) Option { return func(c *config) { c.theme = on } }

// TrackRows records which source row is behind each mark, so that
// [Chart.OnHover] can say. It is off by default because it is not free: every
// layer records where each of its rows landed, every frame.
func TrackRows(on bool) Option { return func(c *config) { c.trackRows = on } }

// Select makes a click on a mark pick the row behind it, and a click on nothing
// clear what was picked. It is off by default.
//
// A picked row gets a ring in **every** view rather than in the one it was
// picked in, because a figure with several cameras is one scene looked at
// several ways — which is the whole reason to draw one from more than one
// angle, and the interaction figure's ADR 0062 named and left to the host.
//
// It implies row tracking. In a projected scene the source row is the entire
// answer a pointer has, so a selection without it would pick nothing at all;
// that is not a default anybody would want overridden.
func Select(on bool) Option { return func(c *config) { c.selects = on } }

// MultiSelect makes every click add to the selection or take its row back out,
// rather than replacing it. It is off by default and does nothing without
// [Select].
func MultiSelect(on bool) Option { return func(c *config) { c.multiSelect = on } }

// Ring sets what the mark round a picked row looks like. The zero value takes
// the theme's label colour at six device units.
//
// Only the look is taken: where the rings go is the selection's, and a Ring
// that named positions would have them overwritten on the next frame.
func Ring(h three.Highlight) Option {
	return func(c *config) { c.ring = three.Highlight{Radius: h.Radius, Color: h.Color, Width: h.Width} }
}

// FrameInterval sets how long a frame has to itself before the next turn is
// drawn. Zero, the default, measures it: the last frame's own cost. A negative
// value draws every event, which is what a test wants and a reader does not.
func FrameInterval(d time.Duration) Option { return func(c *config) { c.interval = d } }

// Detail sets the resolution, as a fraction of the screen's, the chart is drawn
// at while a program has made it coarse with [Chart.SetCoarse]. 0.5 is a
// quarter of the pixels and looks it; it is meant for a chart that is moving
// beside the one a reader is turning, not for one anybody is reading.
//
// Anything outside (0, 1) leaves the chart sharp whatever it is told, which is
// the default. See detail.go.
func Detail(f float32) Option { return func(c *config) { c.detail = f } }

// Cursor sets the pointer shown over the chart. The default leaves the pointer
// alone.
func Cursor(cur desktop.Cursor) Option {
	return func(c *config) {
		if cur != nil {
			c.cursor = cur
		}
	}
}
