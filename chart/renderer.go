package chart

import "fyne.io/fyne/v2"

// renderer is the chart's Fyne renderer: the raster the chart is drawn into,
// and the tooltip that floats over it.
type renderer struct {
	c       *Chart
	objects []fyne.CanvasObject
}

var _ fyne.WidgetRenderer = (*renderer)(nil)

// Layout is where a chart learns its size.
//
// Fyne calls it whenever the widget is resized — BaseWidget.Resize calls
// Layout rather than Refresh — so this is the resize hook, and it is where the
// chart is opened for the first time as well: a plot needs a size, and a
// widget has none until it is laid out.
func (r *renderer) Layout(size fyne.Size) {
	r.c.lock.Lock()
	defer r.c.lock.Unlock()

	if obj := r.c.target.Object(); obj != nil {
		obj.Move(fyne.NewPos(0, 0))
		obj.Resize(size)
	}
	// The pointer layer covers the chart whether or not it is shown, so that
	// making the chart interactive needs no layout of its own.
	r.c.ptr.Move(fyne.NewPos(0, 0))
	r.c.ptr.Resize(size)
	r.c.resize(size)
}

// MinSize is what the widget asks its layout for. It is a size a chart can say
// something at rather than the raster's own one-pixel minimum, which would let
// a box layout collapse the chart to nothing.
func (r *renderer) MinSize() fyne.Size { return r.c.cfg.min }

func (r *renderer) Objects() []fyne.CanvasObject { return r.objects }

// Refresh redraws the chart. It is what BaseWidget.Refresh reaches.
func (r *renderer) Refresh() {
	r.c.lock.Lock()
	defer r.c.lock.Unlock()

	r.c.syncTheme()
	r.c.checkScale()
	r.c.draw()
}

// Destroy closes the chart. A widget that has left the tree keeps no pixels.
func (r *renderer) Destroy() { _ = r.c.Close() }
