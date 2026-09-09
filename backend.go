package fynefigure

import "github.com/timzifer/figure/ir"

// counter wraps the rasterizer's backend to count the frames it painted.
//
// Live.Draw compares the frame it recorded with the last one and returns
// having done nothing when the two are the same — which is the common case for
// a pointer moving over a chart nobody is zooming. A frame it did paint ends
// in Flush. So a Flush is exactly "there is something new on the surface", and
// counting them is what lets [Target.Present] leave the widget alone when
// there is not.
//
// The surface's own pixel generation looks like it would answer the same
// question and does not: it identifies the buffer, so it moves when the buffer
// is reallocated — a resize — and not when something is drawn into it.
//
// # Forwarding
//
// A wrapper hides what it does not declare, and the failure is silent: figure
// asks a backend for its optional interfaces by type assertion, so a wrapper
// that forgot ir.Partial would turn every partial repaint into a full one
// without anything saying so. Damage and Resize are therefore declared here.
// ir.Semantics deliberately is not: the wrapped backend is always the raster
// one, and a raster has nowhere to put a chart's title in words.
type counter struct {
	ir.Backend
	frames uint64

	// w and h are the surface's size in the coordinates a damage rectangle is
	// given in, which is what makes "how much of the frame is this" a question
	// with an answer.
	w, h float32

	// budget is how much of a frame a partial repaint may cover before the
	// whole frame is repainted instead. See [DamageBudget].
	budget float32
}

var (
	_ ir.Backend = (*counter)(nil)
	_ ir.Partial = (*counter)(nil)
	_ ir.Resizer = (*counter)(nil)
)

// Flush completes a frame, which is the one thing this wrapper is for.
func (c *counter) Flush() error {
	c.frames++
	return c.Backend.Flush()
}

// Damage limits the next frame to the given rectangles, or gives up on the
// limit when it would cost more than it saves.
//
// Widening a damage list is what [ir.Partial] permits a backend to do, and
// widening it to the whole frame asks for the plain repaint the frame was
// going to need anyway. What that avoids is a second clip — see
// [DamageBudget], which is where the measurements are.
func (c *counter) Damage(rects []ir.Rect) {
	p, ok := c.Backend.(ir.Partial)
	if !ok {
		return
	}
	if len(rects) == 0 || (c.budget > 0 && c.area(rects) <= c.budget) {
		p.Damage(rects)
		return
	}
	p.Damage(nil)
}

// area is how much of the surface a partial repaint would cover, as a
// fraction. It is the bounding box of the rectangles rather than their sum,
// because the bounding box is what the rasterizer clears and clips to.
func (c *counter) area(rects []ir.Rect) float32 {
	if c.w <= 0 || c.h <= 0 {
		return 1
	}
	box := rects[0]
	for _, r := range rects[1:] {
		box.Min.X = min(box.Min.X, r.Min.X)
		box.Min.Y = min(box.Min.Y, r.Min.Y)
		box.Max.X = max(box.Max.X, r.Max.X)
		box.Max.Y = max(box.Max.Y, r.Max.Y)
	}
	return (box.Dx() * box.Dy()) / (c.w * c.h)
}

// Resize forwards a change of surface size or device pixel ratio.
func (c *counter) Resize(s ir.Surface) error {
	c.w, c.h = float32(s.WidthPx), float32(s.HeightPx)
	if r, ok := c.Backend.(ir.Resizer); ok {
		return r.Resize(s)
	}
	return nil
}
