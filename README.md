# fyne-refract

[![CI](https://github.com/timzifer/fyne-refract/actions/workflows/ci.yml/badge.svg)](https://github.com/timzifer/fyne-refract/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/timzifer/fyne-refract.svg)](https://pkg.go.dev/github.com/timzifer/fyne-refract)
[![MIT](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

[refract](https://github.com/timzifer/refract) charts in a [Fyne](https://fyne.io) app.

```go
p := refract.New(refract.Responsive(true), refract.Title("Signal"))
p.Add(geom.Line(src, geom.X("t"), geom.Y("signal")))

w.SetContent(chart.New(p))
```

That is the whole of it. The widget hovers, drags to pan, zooms about the
pointer, resets the view on a double click, follows its own size and the
application's colours, and shows a tooltip for the mark under the pointer.

```sh
go get github.com/timzifer/fyne-refract
go run github.com/timzifer/fyne-refract/cmd/demo@latest
```

## What is in here

| Package | |
|---|---|
| `fynerefract` | an `ir.Target` that rasterizes a chart and shows it in a `canvas.Raster` |
| `fynerefract/chart` | the widget: the plot, the pointer, the tooltip, the theme, the stream |

The split is refract's own, between `backend/window` and `backend/window/show`:
one draws, the other steers. A backend must not know what a scale or a panel
is, and everything in `chart` is about scales and panels.

## Why it draws the way it does

Fyne's canvas has lines, rectangles, circles, regular polygons, arcs, text and
images. It has no path API — no Bézier segments, no clipping, no affine
transform, no stroke joins or dashes, no rotated text — and its own rasterizer
is under `internal/`. A renderer built on those primitives could not draw a
smoothed line, a polar chart, a clipped panel or a rotated axis label at all.

Writing a second rasterizer instead is what refract's
[ADR 0021](https://github.com/timzifer/refract/blob/main/docs/adr/0021-native-window.md)
rejects: the day it disagrees with the first is the day a chart looks different
on screen than in the file it exports. So this does what refract's native
window does. It rasterizes with `backend/gg` — refract's one rasterizer — and
presents the pixels.

Two things follow, and both are tested:

- **A chart on screen is the PNG beside it.** `p.Render(gg.PNG("chart.png"))`
  and the widget produce identical pixels, because they are the same code.
- **Text is measured with the face it is drawn with**, so labels are placed
  exactly rather than approximated.

`Live.Draw` compares the frame it recorded with the last one and paints nothing
at all when they are the same, so a pointer moving over a chart nobody is
zooming does not repaint the widget. What it does *not* do here is repaint only
the part that changed — see below.

## The widget

```go
c := chart.New(p,
    chart.MinSize(320, 200),
    chart.Detail(0.5),                     // soften while dragging; 1 is the default
    chart.FrameInterval(0),                // 0 is adaptive, <0 draws every event
    chart.Follow(true, false),             // x tracks the data, y stays put
    chart.TrackRows(true),                 // Hit.Row on every hover
    chart.DragMode(refract.DragSelects),   // drag marks out a rectangle
    chart.Brush(&refract.Brush{}),         // what that rectangle looks like
    chart.LegendToggle(true),              // click a legend row to hide a series
    chart.Overlay(&refract.Crosshair{}),   // paint over the finished chart
    chart.TooltipFormat(myFormat),         // or chart.Tooltip(false)
    chart.TooltipLook(myTooltipStyle),     // colours, padding, type
    chart.ThemeFont(true),                 // the app's typeface; see below
    chart.WheelScale(4),                   // zoom per notch
)
```

Events are refract's, not a second set of callbacks:

```go
p.On(refract.Hover, func(ev refract.Event) {
    if ev.Found {
        status.SetText(ev.Series())
    }
})
```

### Selecting, hiding, linking

refract wires none of these itself, and says why in its ADRs 0045 and 0047: a
legend that always toggled, a crosshair nobody asked for and two charts that
always moved together would each be wrong somewhere. The widget offers them as
switches because for a widget they are the common case, and every one of them
is off by default.

A drag can mark out a rectangle instead of panning. The rows under it arrive as
ordinary refract events, one per layer, and the chart draws the band — refract
paints nothing while one is being dragged out, because what the feedback looks
like is the surface's business:

```go
c := chart.New(p, chart.DragMode(refract.DragSelects))
c.Plot().On(refract.Select, func(ev refract.Event) {
    fmt.Println(ev.Hit.Series, len(ev.Rows))
})
```

Selecting turns row tracking on by itself: a selection reads rows out of the
hit index, and an index that was not tracking them holds none.

A click on a legend row can hide the layer it stands for. The axes deliberately
do not move — a toggle is a reading aid, and an axis that rescaled on every
click would make the two readings incomparable:

```go
c := chart.New(p, chart.LegendToggle(true))
c.HideLayer(1, true)   // or ToggleLayer, ShowAllLayers, LayerHidden
```

Two charts are linked by handing one's view to the other, one line each way.
That does not loop: `SetView` is not a reader moving anything and reports
nothing back.

```go
left.OnViewChange(func(v refract.View) { right.SetView(v) })
right.OnViewChange(func(v refract.View) { left.SetView(v) })
```

### Overlays

An overlay paints over a finished chart — a crosshair, rings around a set of
points, a box of text. It is installed once and then moved, because it is a
pointer to a struct whose fields a handler writes:

```go
cross := &refract.Crosshair{}
c.Overlay(cross)
c.Plot().On(refract.Hover, func(ev refract.Event) {
    cross.At, cross.Show = ev.Hit.At, ev.Found
})
```

A hover redraws the chart while an overlay is installed and does not while one
is not, so a chart with no use for one should not install a do-nothing overlay
to keep the code uniform. The chart composes the caller's overlay with the band
of a selection drag, so a chart with both shows both.

### Transitions

A transition is refract's — a keyed join between two tables, blended in data
space — and the clock is the host's. The widget is the clock:

```go
tw, _ := data.NewTween(before, after, "lang", data.Hold("slot"))
tr, _ := c.Transition(tw)
c.Play(tr.Over(400*time.Millisecond).Ease(refract.EaseOut), func() {
    fmt.Println("arrived")
})
```

Frames are advanced by the wall clock on Fyne's goroutine, so a transition of
four hundred milliseconds takes four hundred milliseconds on a machine that
cannot draw sixty frames in that time: it skips the frames it cannot afford
instead of running long. A transition being played belongs to the goroutine
playing it — that is what the completion callback is for, and why nothing on it
should be read from elsewhere while it runs.

### Live data

```go
st := data.NewStream("t", "y").Window(600)
p.Add(geom.Line(st.Source(), geom.X("t"), geom.Y("y")))

c := chart.New(p)
c.Stream(st)
c.Animate(50 * time.Millisecond)
```

The producer appends from wherever it likes; the chart freezes a snapshot
between frames, so it never reads a stream half-written. From a goroutine of
your own, ask for a frame with `c.Redraw()` — every other method here belongs
to Fyne's goroutine.

A sliding window needs two things a static chart does not, and `chart.Follow`
does both for the x axis by default.

A scale is *trained*, and training accumulates: a domain grows and never
shrinks, because a chart of a fixed table must not depend on the order its rows
arrived in. A window whose oldest row leaves every frame is the other case, and
an axis that remembers that row pins the left edge to the first sample the chart
ever saw. So a followed axis is released before each frame and established again
from the rows that are there.

A linear axis also *nices* by default — it rounds its domain outward to whole
tick steps. That frames a still chart well and fights a moving one: the axis
holds while the data slides under it, then jumps a whole tick and holds again,
so the oldest samples visibly leave the chart before the axis admits they are
gone. A followed axis is rebuilt without that rounding, once, keeping everything
else about it.

A followed chart is not dragged or zoomed. That sounds like a restriction and
is the opposite: a sliding window has nothing behind its tip, because the rows
that scrolled off were dropped, so a pan that took the view back would strand
the reader in front of data that no longer exists while the live data marched
off the other side. It would also flicker — refract redraws from inside its own
pan, so a gesture frame would show the dragged view and the next frame would
snap back to the data. `chart.FollowPause(true)` is the other policy: the first
gesture pauses following, the view stays where the reader put it, and the
double click that resets the view hands the chart back. That is what a source
with history rather than a window wants.

Pin the y axis with `scale.Domain`; one that rescales itself every frame makes
two frames impossible to compare by eye.

### Tooltips

A hover shows the series and the values under the pointer. What it says is
`chart.TooltipFormat`, and a label with newlines in it is drawn as several
lines:

```go
chart.TooltipFormat(func(h refract.Hit) string {
    return fmt.Sprintf("%s
%.4g", h.Series, h.Y)
})
```

How it looks is `chart.TooltipLook`, and a caller who wants both per hover
returns a `chart.TooltipContent` instead — from a function, or from any type
implementing `chart.Tooltipper`, which is where formatting with state of its
own belongs:

```go
type reading struct{ unit string }

func (r reading) Tooltip(h refract.Hit) chart.TooltipContent {
    c := chart.TooltipContent{Text: fmt.Sprintf("%.1f %s", h.Y, r.unit)}
    if h.Y < 0 {
        c.Style.Text = color.NRGBA{R: 220, G: 60, B: 60, A: 255}
    }
    return c
}

c := chart.New(p, chart.TooltipWith(reading{unit: "°C"}))
```

Every field of a `chart.TooltipStyle` is optional: what a content leaves zero
comes from `chart.TooltipLook`, and what that leaves zero comes from the Fyne
theme.

The box is drawn by the rasterizer rather than by Fyne's text engine, and that
is the point. Fyne substitutes U+FFFD for a character the theme font has no
glyph for — see `internal/painter/font.go` — and a theme carrying its own font
resource gets no fallback at all, so `≤ ≥ ∞ ±` came out as `�` in the one place
a chart puts numbers in front of a reader. Drawing the tooltip through the same
rasterizer as the axis beside it gives it the same coverage and the same
typeface, and brings multi-line labels with it.

## Pacing

Fyne's desktop driver drains the whole operating-system event queue in one
pass, so a pointer moving at a hundred events a second delivers a hundred of
them — and each one that pans or zooms costs a frame. At tens of milliseconds a
frame that is four times the work there is time for, and the backlog does not
drain: the chart falls further behind the cursor for as long as the drag lasts.

So a frame that would start before the last one has been paid for is not drawn.
The newest position replaces it, and a trailing timer draws whatever is left
over, so a drag that stops still lands where it stopped. A hundred drag events
cost 60 ms of work instead of 2.6 seconds.

Nothing is lost by dropping a pan — refract pans by the distance from the last
position it was told about, so the next one covers the whole way — and nothing
by dropping a zoom, because the deltas are added up and applied together
(a wheel factor is `exp(delta/1000)`, and `exp(a)*exp(b)` is `exp(a+b)`).
Hovering is never paced: it reads the hit index, draws nothing, and costs
microseconds.

The interval is the last frame's own measured cost, so the pacing follows the
machine, the chart's size and the amount of data rather than a guess.
`chart.FrameInterval` pins it, or turns it off.

## Level of detail

A rasterizer's work is per pixel; stretching a small picture over a large area
is the graphics card's, and free. Fyne hands a `canvas.Raster` the pixel size it
is about to paint but uploads whatever size it is given as the texture, and
draws the quad at the widget's size either way — so a frame drawn at half the
resolution is scaled up by the GPU rather than resampled on the processor.

So a chart can be drawn coarse while a reader is dragging or zooming it and
properly once they stop. Per drag frame on a 900x500 chart, 23 ms becomes
7.4 ms at `chart.Detail(0.5)`.

It is **off by default**, because it is visible: a chart in motion is soft
until it is let go, and that is a trade a reader should be offered rather than
given. Reach for it when a chart is large, the data heavy or the machine slow —
and note that the GPU tier below makes the same frame cost 5 ms without
softening anything, so try that first. A chart nobody is touching is always
drawn at full resolution.

## What a frame costs

Rasterizing is the whole of it, and it is not cheap. On a 900x480 chart, per
frame, on a Ryzen 7 5800H:

| | |
|---|---|
| empty chart, axes and titles only | 20 ms |
| a 600-point line, whole frame | 25 ms |
| panning a 4000-point line | 45 ms |
| the same at half resolution, with chart.Detail(0.5) | 21 ms |
| a sliding stream at half resolution | 8 ms |
| a hover that finds a mark, no repaint | 3 µs |

So an interactive chart runs at twenty to thirty frames a second, and a hover
costs nothing because it paints nothing. That is the rasterizer's price, and it
is why the pacing above exists rather than being a nicety.

There is a GPU tier, in `fyne-refract/gpu`, and it is worth having:

| | CPU | GPU |
|---|---|---|
| a sliding stream, whole frame | 25 ms | 7 ms |
| panning a 4000-point line | 45 ms | 5 ms |

One blank import turns it on:

```go
import _ "github.com/timzifer/fyne-refract/gpu"
```

A machine with no usable device falls back to the CPU rasterizer, and
`gpu.Enabled` reports which way it went — refract's tier proves the accelerator
puts ink in a buffer before keeping it, so that is an answer about drawing
rather than about registration.

The two tiers do not share a coverage filler, so they do not agree pixel for
pixel: the difference is the antialiasing on the edge of everything drawn,
about one part in 255 on average. The package's parity test measures it.

Refract offers to tell a backend *where* a frame changed so that only that part
is repainted. It is off here, because measured it is six times slower: the
rasterizer clears the damaged box and clips every drawing call to it, and its
clip is a mask rasterized across the surface and then sampled per pixel — on
top of the clip a panel already puts there. A stream sliding left costs 29 ms a
frame whole and 180 ms partial. `fynerefract.DamageBudget` turns it back on for
a chart whose changes really are confined to a corner; `go test -bench=Frame`
is the measurement.

### Colours and type

The chart reads the application's background colour and picks refract's light
or dark theme to match, in the page colour and the text size the Fyne theme
asks for. `chart.FollowTheme(false)` turns that off.

The labels are drawn in refract's own fonts rather than the application's, and
that is deliberate. Fyne renders text through a shaper that falls back: a
character its theme font has no glyph for is drawn from another font, so the
label appears. The rasterizer that draws a chart is handed one font and has no
fallback, so the same character is drawn as nothing at all — silently, leaving a
gap. Fyne's theme font is NotoSans-Regular, which has no glyph for `≤`, `≥` or
`∞`, so a chart titled `30° ≤ x` would keep the degree sign and lose the rest.
`chart.ThemeFont(true)` matches the application's typeface for a chart whose
labels are plain enough to carry it; the default also keeps the widget
comparable pixel for pixel with an exported PNG.

## Building

```sh
go build ./... && go vet ./... && go test ./...
gofmt -l .          # must print nothing
go run ./cmd/demo   # needs a display
```

The tests need no display: Fyne's software painter draws the widget, and what
is asserted is what refract reports — the size it was laid out at, the domain
after a zoom, the frames it painted — rather than how it looks.

On Linux the demo needs the headers Fyne's desktop driver is built against:
`libgl1-mesa-dev xorg-dev libxkbcommon-dev`. The library packages themselves
build with `CGO_ENABLED=0`.

See [CONTRIBUTING.md](CONTRIBUTING.md) for the layout, the benchmarks behind
the defaults, and what to know before changing them.

## Versions

`fyne.io/fyne/v2` v2.7.3, `github.com/timzifer/refract` v1.7.0 and its raster
backend at the same tag, pinned exactly — a release of this bridge is validated
against one release of each and says which.

refract and its rasterizer are cgo-free; Fyne's desktop driver is not, so a
program using this needs cgo the way any Fyne program does. Only the packages
here and the tests build with `CGO_ENABLED=0` — the widget does, the window it
goes in does not.
