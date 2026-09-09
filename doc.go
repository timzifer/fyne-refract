// Package fynefigure draws figure charts into a Fyne canvas object.
//
//	t := fynefigure.New()
//	live, err := p.Live(t)
//	// ... live.Draw(); t.Present() ...
//	container.NewStack(t.Object())
//
// It is an [ir.Target] like any other, and what it adds is that the pixels end
// up in a Fyne widget tree. The package next door, fynefigure/chart, is what
// joins one to a *figure.Plot and turns Fyne's mouse events into hovers, pans
// and zooms — a backend must not know what a scale or a panel is, so steering
// is a separate package for the same reason figure keeps backend/window and
// backend/window/show apart.
//
// # Why a raster and not canvas objects
//
// Fyne's canvas package draws lines, rectangles, circles, regular polygons,
// arcs, text and images. It has no path API: no Bézier segments, no clipping,
// no affine transform, no stroke joins or dashes, no rotated text. Its own
// rasterizer is under internal/ and cannot be imported. So an ir.Backend built
// on Fyne primitives could not draw a smoothed line, a polar chart, a clipped
// panel or a rotated axis label at all.
//
// The alternative — a second rasterizer of figure's own, drawing into an
// image — is the thing figure's ADR 0021 rejects: the day it disagrees with
// the first is the day a chart looks different on screen than in the file it
// exports. So this package does what the native window does. It rasterizes
// with backend/gg, which is figure's one rasterizer, and presents the result.
// A chart in a Fyne widget is the same pixels as the PNG beside it, and text
// is measured with the face it is drawn with rather than approximated.
package fynefigure
