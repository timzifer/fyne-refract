package chart

import (
	"fmt"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver/desktop"
	"github.com/timzifer/refract"
)

// Option configures a [Chart] at construction.
type Option func(*config)

type config struct {
	detail    float32
	interval  time.Duration
	pause     bool
	followX   bool
	followY   bool
	min       fyne.Size
	tooltip   bool
	format    func(refract.Hit) string
	theme     bool
	font      bool
	trackRows bool
	wheel     float64
	cursor    desktop.Cursor
}

func defaults() config {
	return config{
		detail:  1,
		followX: true,
		min:     fyne.NewSize(240, 160),
		tooltip: true,
		format:  DefaultTooltip,
		theme:   true,
		wheel:   DefaultWheelScale,
		cursor:  desktop.CrosshairCursor,
	}
}

// DefaultWheelScale converts one unit of Fyne's scroll delta into the pixels
// [refract.WheelFactor] expects. Fyne reports a notch of a mouse wheel as a
// handful of units where a browser reports it as a line of text, so a chart
// that zoomed by the raw number would barely move.
const DefaultWheelScale = 4

// MinSize sets the smallest size the widget asks its layout for. The default
// is 240x160: a chart with axes and a legend has nothing useful to show below
// that, and a widget that reports the raster's own minimum collapses to
// nothing in a box layout.
func MinSize(w, h float32) Option {
	return func(c *config) { c.min = fyne.NewSize(w, h) }
}

// Tooltip turns the hover tooltip on or off. It is on by default.
func Tooltip(on bool) Option { return func(c *config) { c.tooltip = on } }

// TooltipFormat replaces what the tooltip says. It is called for the mark
// under the pointer; returning an empty string hides the tooltip for that
// mark.
func TooltipFormat(fn func(refract.Hit) string) Option {
	return func(c *config) {
		if fn != nil {
			c.format = fn
		}
	}
}

// DefaultTooltip is what a tooltip says unless [TooltipFormat] says otherwise:
// the series name, if the layer has one, and the values under the pointer.
func DefaultTooltip(h refract.Hit) string {
	if h.Series == "" {
		return fmt.Sprintf("x %.4g\ny %.4g", h.X, h.Y)
	}
	return fmt.Sprintf("%s\nx %.4g\ny %.4g", h.Series, h.X, h.Y)
}

// Follow makes the named axes track the rows the chart currently holds instead
// of every row it has ever been shown. It applies to a chart with a
// [Chart.Stream] and to no other, and the default is the x axis alone, which
// is what a time series wants.
//
// Two things stand between a sliding window and an axis that follows it, and
// this handles both.
//
// A scale is *trained*, and training accumulates: a domain only ever grows,
// because that is what a chart of a fixed table wants — every render sees the
// same rows, and the axis must not depend on the order they arrived in. A
// window whose oldest row leaves on every frame is the other case, so a
// followed axis is released before each frame and established again from the
// rows that are there.
//
// And a linear axis *nices* by default: it rounds its domain outward to whole
// tick steps. That frames a still chart well and fights a moving one — the
// axis holds while the data slides under it, then jumps a tick and holds
// again, so the oldest samples visibly leave the chart before the axis admits
// they are gone. A followed axis is therefore rebuilt from its own description
// with that rounding off, once, keeping everything else about it. An axis
// pinned to a domain is left alone — a pinned domain is never niced — and so
// is one carrying a formatter, which a description cannot hold.
//
// The y axis is not followed by default: a live chart is far easier to read
// against a fixed scale, and pinning one with scale.Domain is the usual answer.
//
// Following stops the moment a reader zooms or pans — a view someone dragged
// into place is not something to take away from them — and starts again on the
// double click that resets the view.
func Follow(x, y bool) Option {
	return func(c *config) { c.followX, c.followY = x, y }
}

// Detail sets how much of the screen's resolution a chart is drawn at while a
// reader is dragging or zooming it. The default is 1: full resolution, always.
//
// A rasterizer's work is per pixel; stretching a small picture over a large
// area is the graphics card's, and free. So a chart can be drawn coarser while
// a gesture is in flight and properly once it ends, and on the CPU rasterizer
// that is worth a great deal — 23 ms a frame becomes 7 ms at 0.5, which is the
// difference between twenty frames a second and fifty.
//
// It is off by default because it is visible: a chart being dragged is soft
// until it is let go, and that is a trade a reader should be offered rather
// than given. Reach for it when a chart is large, or the data heavy, or the
// machine slow — and note that the GPU tier in fyne-refract/gpu makes the same
// frame cost 5 ms without softening anything, so try that first.
//
// Values outside (0, 1] mean the same as 1. A chart nobody is touching is
// always drawn at full resolution.
func Detail(f float32) Option {
	return func(c *config) { c.detail = f }
}

// FrameInterval sets how much time a frame is given to itself while a reader
// is dragging or zooming.
//
// Input arrives far faster than a chart can be drawn — see pace.go — so a
// frame that would start before the last one has been paid for is held back
// and replaced by the next, and the newest one is drawn when the interval is
// up. The default, which is what a zero here means, is the last frame's own
// measured cost: a chart that draws in three milliseconds is not held to sixty
// a second, and one that takes fifty is not asked for more than twenty.
//
// Name one to pin the rate — 16ms for sixty a second on a chart that can keep
// up — or pass a negative duration to turn the pacing off and draw every event
// as it arrives, which is what a test that wants one frame per call does.
func FrameInterval(d time.Duration) Option {
	return func(c *config) { c.interval = d }
}

// FollowPause decides what happens when a reader drags or zooms a chart whose
// axes follow the data. It is off by default.
//
// Off, the chart stays in charge: a pan or a zoom that would move a followed
// axis is ignored, and the chart goes on tracking its data. That is the right
// default for a sliding window, because there is nothing behind the tip to pan
// to — the rows that scrolled off were dropped, so a drag that took the view
// back would strand the reader in front of data that no longer exists while
// the live data marched off the other side.
//
// On, the reader takes over: the first pan or zoom pauses following, the view
// stays where they put it, and the double click that resets the view hands the
// chart back to the data. That is what a chart with history rather than a
// window wants — one whose source keeps everything it has been given.
//
// It has no effect on a chart that follows nothing, which is every chart
// without a [Chart.Stream]: those pan and zoom as they always did.
func FollowPause(on bool) Option { return func(c *config) { c.pause = on } }

// FollowTheme makes the chart switch between refract's light and dark themes
// with Fyne's own. It is on by default.
//
// Switching rebuilds the chart, which forgets where it was zoomed to — a fresh
// start is what a change of theme is.
func FollowTheme(on bool) Option { return func(c *config) { c.theme = on } }

// ThemeFont draws the chart's labels in the application's typeface, read from
// the Fyne theme. It is off by default, and the reason is worth knowing before
// turning it on.
//
// Fyne renders text through a shaper that falls back: a character its theme
// font has no glyph for is drawn from another font, so the label appears. The
// rasterizer that draws a chart is handed one font and has no fallback, so the
// same character is drawn as nothing at all — silently, leaving a gap.
//
// That is not hypothetical. Fyne's own theme font is NotoSans-Regular, which
// has no glyph for U+2264, U+2265 or U+221E: a chart titled "30° ≤ x" loses
// the ≤ and keeps the degree sign, and an axis labelled in ∞ loses that. Those
// are exactly the characters a chart reaches for.
//
// So the default is the fonts every other refract raster uses. They cover more,
// and they are what makes a chart on screen comparable pixel for pixel with the
// PNG the same plot exports. Turn this on when the chart's labels are plain
// enough for the theme's font to carry, and matching the application's
// typeface matters more.
func ThemeFont(on bool) Option { return func(c *config) { c.font = on } }

// TrackRows records which source row is behind each mark, so that a hover can
// report it. It is off by default because it is not free — see
// [refract.Live.TrackRows].
func TrackRows(on bool) Option { return func(c *config) { c.trackRows = on } }

// WheelScale sets how much of a zoom one unit of Fyne's scroll delta is worth.
// The default is [DefaultWheelScale]; a negative value inverts the direction.
func WheelScale(f float64) Option {
	return func(c *config) {
		if f != 0 {
			c.wheel = f
		}
	}
}

// Cursor sets the pointer shown over the chart. The default is a crosshair;
// pass [desktop.DefaultCursor] to leave the pointer alone.
func Cursor(cur desktop.Cursor) Option {
	return func(c *config) {
		if cur != nil {
			c.cursor = cur
		}
	}
}
