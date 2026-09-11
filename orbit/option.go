package orbit

import (
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver/desktop"
)

// Option configures a [Chart].
type Option func(*config)

type config struct {
	min       fyne.Size
	perPixel  float64
	wheel     float64
	together  bool
	theme     bool
	trackRows bool
	interval  time.Duration
	cursor    desktop.Cursor
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

// MinSize sets the smallest size the widget asks its layout for. The default
// is 240x160.
func MinSize(w, h float32) Option {
	return func(c *config) { c.min = fyne.NewSize(w, h) }
}

// PerPixel sets how far a pixel of drag turns the camera, in radians. The
// default is [DefaultPerPixel]; a negative value turns the scene the other way
// round, which some readers expect of a globe.
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

// FrameInterval sets how long a frame has to itself before the next turn is
// drawn. Zero, the default, measures it: the last frame's own cost. A negative
// value draws every event, which is what a test wants and a reader does not.
func FrameInterval(d time.Duration) Option { return func(c *config) { c.interval = d } }

// Cursor sets the pointer shown over the chart. The default leaves the pointer
// alone.
func Cursor(cur desktop.Cursor) Option {
	return func(c *config) {
		if cur != nil {
			c.cursor = cur
		}
	}
}
