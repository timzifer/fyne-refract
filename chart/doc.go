// Package chart shows a refract plot in a Fyne widget.
//
//	p := refract.New(refract.Responsive(true), refract.Title("Signal"))
//	p.Add(geom.Line(src, geom.X("t"), geom.Y("y")))
//
//	w.SetContent(chart.New(p))
//
// The widget hovers, clicks, zooms about the pointer, pans on a drag, resets
// the view on a double click and follows its own size — all of which is
// [refract.Input] driving [refract.Live], the same state machine the browser
// and the native window use. What this package adds is the wiring, and a
// tooltip, a theme that follows Fyne's, and a way to keep a stream moving.
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
// # Why this is not in package fynerefract
//
// Because wiring input is not drawing. A backend consumes IR and must not know
// what a scale or a panel is, and everything here is about scales and panels;
// package fynerefract draws, and this steers. refract makes the same split
// between backend/window and backend/window/show.
//
// # Threading
//
// Every method here is called on Fyne's own goroutine, which is where the
// widget's events arrive. A chart driven from a goroutine of your own —
// a producer appending samples, a ticker — calls [Chart.Redraw], which is the
// one method that may be called from anywhere.
package chart
