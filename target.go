package fynerefract

import (
	"image"
	"sync"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	ggbackend "github.com/timzifer/refract/backend/gg"
	"github.com/timzifer/refract/ir"
)

// Target is a render destination that draws into memory and shows the result
// in a Fyne canvas object.
//
// It is [ggbackend.Surface] with a Fyne front end. The surface keeps its
// pixels, so the backend it opens implements ir.Resizer and ir.Partial: a
// resized chart is repainted in place, and a live one repaints only where it
// changed. What this type adds is [Target.Object], the object to put in a
// widget tree, and [Target.Present], which shows the frame that was just
// drawn.
//
// A Target draws one chart at a time and is not safe for concurrent use.
type Target struct {
	// mu guards the surface. The rasterizer is single-goroutine and so is the
	// buffer it draws into, and Fyne reads that buffer from its painter — which
	// on a desktop driver is the goroutine that drew it, and under the test
	// driver is not. Rather than depend on which, every path in or out of the
	// surface goes through here, including the drawing itself: see Render.
	mu     sync.Mutex
	surf   *ggbackend.Surface
	raster *canvas.Raster
	cfg    config

	// back counts the frames the rasterizer painted, and shown is how many of
	// them the widget has been shown, so that a frame nobody painted costs no
	// repaint.
	back  *counter
	shown uint64

	// geometry is told the pixel size the painter asked for whenever that
	// disagrees with the size the chart was rasterized at. See Object.
	geometry func(widthPx, heightPx int)

	// blank stands in before the first frame, because Fyne's painter has no
	// answer for a raster that generates nothing.
	blank *image.RGBA
}

var _ ir.Target = (*Target)(nil)

// New returns a target that has not been opened yet. Nothing is rasterized
// until a chart is drawn into it.
func New(opts ...Option) *Target {
	c := build(opts)
	t := &Target{cfg: c, surf: ggbackend.NewSurface(c.gg...)}
	t.raster = canvas.NewRaster(t.generate)
	t.raster.ScaleMode = c.scale
	return t
}

// Open prepares the target for a chart of the given size. It is [ir.Target]'s
// half of the contract and is called by refract, not by a caller.
func (t *Target) Open(widthPx, heightPx int, dpr float64) (ir.Backend, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	b, err := t.surf.Open(widthPx, heightPx, dpr)
	if err != nil {
		return nil, err
	}
	t.back, t.shown = &counter{
		Backend: b,
		w:       float32(widthPx),
		h:       float32(heightPx),
		budget:  t.cfg.budget,
	}, 0
	return t.back, nil
}

// Close releases the pixel buffer. The frame is not available afterwards.
//
// Plot.Render closes the target it was given, so rendering a chart once into a
// Target leaves nothing to show — a chart in a widget is drawn through
// Plot.Live, which keeps its target open and redraws into it frame after
// frame. That is the difference between a document and a surface, and it is
// why this type is one.
func (t *Target) Close() error {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.surf.Close()
}

// Render runs fn with the surface held, and is how a frame is drawn.
//
// Everything that draws into the surface goes through here, so that a frame
// cannot be painted while Fyne is reading the last one. fn is whatever
// rasterizes — Live.Draw, Live.Resize, Live.Rescale — and its error is
// returned unchanged.
//
// It must not be called from inside another Render, and fn must not reach back
// into the target: this is a plain lock, not a reentrant one.
func (t *Target) Render(fn func() error) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	return fn()
}

// SetFont replaces the typeface the rasterizer draws labels with, keeping the
// canvas object the chart is already shown in.
//
// A font is fixed when a rasterizer is made, so this makes a new one — which
// is why it closes the surface, and why the chart drawn into it has to be
// opened again afterwards. What it deliberately does not replace is
// [Target.Object]: a widget holds that object, and handing it a different one
// would leave it showing a chart nothing draws into any more.
//
// Pass all three as nil to go back to the rasterizer's own fonts.
func (t *Target) SetFont(regular, bold, italic []byte) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	err := t.surf.Close()
	if len(regular) == 0 {
		t.cfg.gg = nil
	} else {
		t.cfg.gg = []ggbackend.Option{ggbackend.WithFont(regular, bold, italic)}
	}
	t.surf = ggbackend.NewSurface(t.cfg.gg...)
	t.back, t.shown = nil, 0
	return err
}

// Object returns the canvas object the chart is shown in.
//
// It is a [canvas.Raster], so Fyne asks it for an image at the size it is
// about to paint, in *physical* pixels — which is the size the chart should be
// rasterized at, device pixel ratio included. The generator never draws: it
// hands back the frame that is already there. When the painter asks for a size
// the chart was not rasterized at — the frame after a window moves to a
// display with a different ratio — the previous frame is stretched for that
// one paint and the size is reported through [Target.OnGeometry], so that
// whoever owns the chart can rasterize the next one correctly.
func (t *Target) Object() fyne.CanvasObject { return t.raster }

// OnGeometry registers a callback for the pixel size Fyne is painting at, when
// that is not the size the current frame was rasterized at.
//
// It is called from the painter, which is the goroutine Fyne draws on, so a
// handler must not draw a chart from it — it should ask for one, and let the
// redraw happen in its turn.
func (t *Target) OnGeometry(fn func(widthPx, heightPx int)) { t.geometry = fn }

// Present shows the frame that was last drawn.
//
// It is what to call after Live.Draw, and it is cheap when nothing changed:
// Live paints nothing when a frame is identical to the last, and a frame
// nobody painted is not handed to Fyne. So a pointer moving over a chart that
// is not being zoomed costs a comparison of two integers rather than a texture
// upload.
//
// The frame Fyne is handed comes out of the rasterizer's buffer, and the
// painter reads it while it uploads the texture. So the surface is held
// whenever it is drawn into or read from — see [Target.Render] — rather than
// resting on the drawing and the painting being the same goroutine, which they
// are on a desktop driver and are not under Fyne's test one.
func (t *Target) Present() {
	t.mu.Lock()
	if t.back == nil || t.back.frames == t.shown {
		t.mu.Unlock()
		return
	}
	t.shown = t.back.frames
	t.mu.Unlock()

	// Outside the lock: Fyne may paint from here, and painting reads the
	// surface through the generator, which takes it.
	canvas.Refresh(t.raster)
}

// Size reports the logical size of the chart being drawn, in
// device-independent pixels, and the device pixel ratio its buffer is scaled
// by. It is zero before the first chart is opened.
func (t *Target) Size() (w, h int, dpr float64) {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.surf.Size()
}

// Image returns the pixels of the last frame, or nil before the first one.
//
// The image is the rasterizer's own buffer, so it is valid until the next
// frame overwrites it. A caller keeping one past that must copy it. It is what
// a test compares and what an export of exactly what is on screen would read.
func (t *Target) Image() image.Image {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.surf.Image()
}

// Frames reports how many frames have been painted into the surface.
//
// It is not a frame number: refract paints nothing for a frame identical to
// the one before it, so this counts the frames that changed something. It is
// what a test asserts on, and what tells a caller whether the last draw did
// any work.
func (t *Target) Frames() uint64 {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.back == nil {
		return 0
	}
	return t.back.frames
}

// generate is the raster's source of pixels. It draws nothing.
func (t *Target) generate(widthPx, heightPx int) image.Image {
	t.mu.Lock()
	defer t.mu.Unlock()

	img := t.surf.Image()
	if img == nil {
		return t.blankAt(widthPx, heightPx)
	}
	if b := img.Bounds(); b.Dx() != widthPx || b.Dy() != heightPx {
		if t.geometry != nil && widthPx > 0 && heightPx > 0 {
			t.geometry(widthPx, heightPx)
		}
	}
	return img
}

// blankAt is what a raster shows before there is a chart. Fyne's painter has
// no answer for a generator that returns nil.
func (t *Target) blankAt(widthPx, heightPx int) image.Image {
	if widthPx <= 0 || heightPx <= 0 {
		widthPx, heightPx = 1, 1
	}
	if t.blank == nil || t.blank.Bounds().Dx() != widthPx || t.blank.Bounds().Dy() != heightPx {
		t.blank = image.NewRGBA(image.Rect(0, 0, widthPx, heightPx))
	}
	return t.blank
}
