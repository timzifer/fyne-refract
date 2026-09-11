package orbit

import (
	"fyne.io/fyne/v2"
	"github.com/timzifer/figure/three"
	"github.com/timzifer/fyne-figure/internal/look"
)

// renderer is the chart's Fyne renderer: the raster the scene is drawn into.
type renderer struct {
	c       *Chart
	objects []fyne.CanvasObject
}

var _ fyne.WidgetRenderer = (*renderer)(nil)

// Layout is where a chart learns its size, and where it is opened for the
// first time: a plot needs a size, and a widget has none until it is laid out.
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

// MinSize is what the widget asks its layout for.
func (r *renderer) MinSize() fyne.Size { return r.c.cfg.min }

func (r *renderer) Objects() []fyne.CanvasObject { return r.objects }

// Refresh redraws the chart. It is what BaseWidget.Refresh reaches, and where
// Fyne has already told the widget its theme may have changed.
func (r *renderer) Refresh() {
	r.c.lock.Lock()
	defer r.c.lock.Unlock()

	r.c.syncTheme()
	r.c.checkScale()
	r.c.draw()
}

// Destroy closes the chart. A widget that has left the tree keeps no pixels.
func (r *renderer) Destroy() { _ = r.c.Close() }

// syncTheme follows Fyne's colours and text size, and draws the scene again in
// them when either moved.
func (c *Chart) syncTheme() {
	if !c.cfg.theme {
		return
	}
	if look.Read(c.Theme()) == c.themed {
		return
	}
	if c.live != nil {
		c.reopen()
	}
}

// applyTheme puts figure's own light or dark theme on the plot, in the page
// colour and at the text size Fyne asks for. It is three.Theme applied to the
// plot directly: an Option is an ordinary function, and a scene whose
// surroundings changed colour has not become a different scene.
func (c *Chart) applyTheme() {
	if !c.cfg.theme {
		return
	}
	c.themed = look.Read(c.Theme())
	three.Theme(c.themed.Theme())(c.plot)
}

// reopen draws the plot again from its specification, keeping what the reader
// turned each view to.
//
// A three.Live works on its own copy of the plot, taken when it was opened, so
// a new theme reaches it only through a new one. The target survives, because
// the widget is holding its canvas object.
func (c *Chart) reopen() {
	size := fyne.NewSize(float32(c.w), float32(c.h))
	c.cameras = c.cameras[:0]
	for i := range c.live.ViewCount() {
		c.cameras = append(c.cameras, c.live.CameraOf(i))
	}
	c.stopGesture()
	// Closing a Live closes the target it was given; opening the next one
	// opens it again.
	_ = c.live.Close()
	c.live = nil
	c.resize(size)
}
