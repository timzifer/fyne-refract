// Package orbit shows a three-dimensional figure plot in a Fyne widget, and
// turns it under the pointer.
//
//	sc := three.NewScene(three.XTitle("x"), three.YTitle("y"), three.ZTitle("z"))
//	sc.Add(three.Surface(src, geom.X("x"), geom.Y("y"), geom.Z("z")))
//
//	p := three.New(three.Title("Response")).Scene(sc)
//	w.SetContent(orbit.New(p, orbit.Interactive(true)))
//
// The widget follows its own size and the application's colours, as
// [chart.Chart] does. Made [Interactive], a drag takes hold of the view it
// started in and turns it — the side facing the reader follows the pointer —
// the wheel dollies it, and a double click puts every view back at the camera
// its author chose.
//
// # Still until asked
//
// A scene is a picture until it is told otherwise, for the reason package
// chart gives: a widget that took the pointer would take the wheel and the
// drag from the scroll container around it. [Interactive] lets a reader at it
// from construction, and [Chart.SetInteractive] does the same, or takes it
// back, for a chart already on screen.
//
// # Why this is not package chart
//
// Because a [three.Plot] is not a figure.Plot, and turning one is not panning
// one. A flat chart is steered by figure.Input, a state machine figure owns
// that turns pointer positions into zooms and pans of its scales. A projected
// scene has no screen axes to pan through, and figure deliberately ships no
// loop for it: a camera is a value, [three.Orbit] and [three.Dolly] are pure
// functions from one to another, and turning the scene is the host's loop
// (figure's ADR 0057). This package is that loop for a Fyne host.
//
// That is also why the constants are here rather than in figure. How many
// radians a pixel of drag is worth — [PerPixel] — is a statement about how the
// interaction feels, and feel belongs to whoever owns the input layer.
//
// # Several views
//
// A plot with several views — a three-quarter, a plan and two profiles of one
// scene — turns the view a drag or a wheel started in and leaves the others
// where they are, which is what a reader comparing a turned view with a fixed
// one needs. [Together] turns every view by the same amount instead, which
// keeps a front / top / side figure a set of related angles.
//
// Two widgets are linked by handing one's camera to the other, one line each
// way. It does not loop: [Chart.SetCamera] is not a reader moving anything and
// reports nothing back.
//
//	left.OnCamera(func(i int, cam three.Camera) { _ = right.SetCamera(i, cam) })
//	right.OnCamera(func(i int, cam three.Camera) { _ = left.SetCamera(i, cam) })
//
// # Pointing at things
//
// A projected mark has no x and y to read back: a device point in a turned
// cube does not resolve to a pair of values. So a hover reports the hit —
// which layer, which view — and, with [TrackRows], the source row behind it,
// which is the whole answer a pointer has in three dimensions. See
// [Chart.OnHover].
//
// # Threading
//
// Every method here is called on Fyne's own goroutine, which is where the
// widget's events arrive, except [Chart.Redraw], which may be called from
// anywhere. Callbacks run on that goroutine while the chart is held, so one
// must not call back into the chart it was registered on; calling into another
// chart — the link above — is what they are for.
package orbit
