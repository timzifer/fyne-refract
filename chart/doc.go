// Package chart shows a figure plot in a Fyne widget.
//
//	p := figure.New(figure.Responsive(true), figure.Title("Signal"))
//	p.Add(geom.Line(src, geom.X("t"), geom.Y("y")))
//
//	w.SetContent(chart.New(p, chart.Interactive(true)))
//
// The widget follows its own size and the application's theme. Made
// [Interactive], it also hovers, clicks, zooms about the pointer, pans on a
// drag and resets the view on a double click — all of which is [figure.Input]
// driving [figure.Live], the same state machine the browser and the native
// window use. What this package adds is the wiring, and a tooltip, a theme
// that follows Fyne's, and a way to keep a stream moving.
//
// # Still until asked
//
// A chart is a picture until it is told otherwise. It takes no pointer events
// at all — not a hover, not a drag, not the wheel — so a chart placed in a
// scroll container, a list or a form scrolls with it and leaves every gesture
// to what is around it. [Interactive] lets a reader at it from construction,
// and [Chart.SetInteractive] does the same, or takes it back, for a chart
// already on screen.
//
// # Pointing at things
//
// Beyond the gestures every chart has, four things a reader can do are wired
// here and switched off by default, because figure deliberately wires none of
// them: a legend that always toggled and two charts that always moved together
// would each be wrong somewhere.
//
//   - [DragMode] makes a drag mark out a rectangle instead of panning, and
//     [figure.Select] reports the rows under it. The band is drawn here —
//     see [Brush] — because figure paints nothing while one is dragged out.
//   - [LegendToggle] makes a click on a legend row hide the layer it stands
//     for. [Chart.HideLayer] and the calls beside it are the same thing
//     without the pointer.
//   - [Chart.OnViewChange] and [Chart.SetView] link one chart to another.
//   - [Overlay] paints over the finished chart — a crosshair, a highlight, a
//     box of text.
//
// # Transitions
//
// A transition is figure's, and the clock is the host's: [Chart.Transition]
// builds one over the chart's rows and [Chart.Play] drives it to its end on
// Fyne's goroutine, frame by frame, by the wall clock.
//
// # Tooltips
//
// What a hover says is [TooltipFormat]; what it looks like is [TooltipLook];
// and a caller who wants both per hover returns a [TooltipContent], from a
// function given to [TooltipContentFunc] or from a [Tooltipper] given to
// [TooltipWith].
//
// The box is drawn by the same rasterizer as the chart's own labels rather
// than by Fyne's text engine, so it carries the symbols a chart reaches for —
// Fyne draws U+FFFD for a character the theme font has no glyph for, and its
// own theme font has none for U+2264.
//
// A hover in the margins finds a legend row, a colourbar or a size key rather
// than a mark. Those carry no X and no Y, so the tooltip says nothing about
// them: a box reading "x 0, y 0" beside a series name would be a lie about
// where the pointer is. A caller who does want to say something about one
// handles [figure.Hover] and reads [figure.Hit.Kind].
//
// # Why this is not in package fynefigure
//
// Because wiring input is not drawing. A backend consumes IR and must not know
// what a scale or a panel is, and everything here is about scales and panels;
// package fynefigure draws, and this steers. figure makes the same split
// between backend/window and backend/window/show.
//
// # Threading
//
// Every method here is called on Fyne's own goroutine, which is where the
// widget's events arrive. A chart driven from a goroutine of your own —
// a producer appending samples, a ticker — calls [Chart.Redraw], which is the
// one method that may be called from anywhere.
//
// A [figure.Transition] being played is the other way round: it belongs to
// the goroutine playing it, and nothing on it — not even
// [figure.Transition.Done] — may be read from elsewhere while [Chart.Play]
// runs. That is what Play's completion callback is for.
package chart
