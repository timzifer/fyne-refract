package chart

import (
	"errors"
	"image"
	"image/color"
	"image/draw"
	"math"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	fynetheme "fyne.io/fyne/v2/theme"
	"github.com/timzifer/refract"
	ggbackend "github.com/timzifer/refract/backend/gg"
	"github.com/timzifer/refract/ir"
)

// TooltipStyle is how a tooltip is drawn. It is the styling half of a
// [TooltipContent], and every field is optional: a zero field takes the
// chart's default, which comes from [TooltipLook] if one was given and from
// the Fyne theme otherwise.
//
// The two booleans are the exception to "zero means default", because false is
// also an answer: a style that sets Bold draws bold, and one that leaves it
// alone inherits.
type TooltipStyle struct {
	// Text, Background and Border are the label's colour, the box's fill and
	// its outline. A nil colour is inherited; a fully transparent one draws
	// nothing, which is how a tooltip is given no border or no box.
	Text       color.Color
	Background color.Color
	Border     color.Color

	// BorderWidth is the outline's width in device-independent pixels. A
	// negative width draws no outline at all, which is the way to drop an
	// inherited one.
	BorderWidth float32

	// Corner is the box's corner radius, and Padding the space between the
	// text and the box's edge. A negative Corner squares the corners.
	Corner  float32
	Padding float32

	// FontSize is the label's em size, Bold and Italic its face, and
	// LineSpacing the distance between the baselines of a multi-line label as
	// a multiple of the line height.
	FontSize    float32
	LineSpacing float32
	Bold        bool
	Italic      bool
}

// over returns s laid over base: every field s left zero keeps base's.
func (s TooltipStyle) over(base TooltipStyle) TooltipStyle {
	out := base
	if s.Text != nil {
		out.Text = s.Text
	}
	if s.Background != nil {
		out.Background = s.Background
	}
	if s.Border != nil {
		out.Border = s.Border
	}
	if s.BorderWidth != 0 {
		out.BorderWidth = s.BorderWidth
	}
	if s.Corner != 0 {
		out.Corner = s.Corner
	}
	if s.Padding != 0 {
		out.Padding = s.Padding
	}
	if s.FontSize != 0 {
		out.FontSize = s.FontSize
	}
	if s.LineSpacing != 0 {
		out.LineSpacing = s.LineSpacing
	}
	out.Bold = out.Bold || s.Bold
	out.Italic = out.Italic || s.Italic
	return out
}

// TooltipContent is one tooltip: what it says and, optionally, how it is
// drawn. An empty Text hides the tooltip for that hover.
//
// The text may contain newlines, and each line is drawn as its own line.
type TooltipContent struct {
	Text  string
	Style TooltipStyle
}

// Tooltipper builds the tooltip for a hover. It is the extension point for a
// caller who wants more than a string: a type that holds the units, the
// formatting or the palette a chart's tooltips need can implement this and be
// passed to [TooltipWith].
//
// [TooltipFormat] and [TooltipContentFunc] are the function-shaped shortcuts
// to the same thing.
type Tooltipper interface {
	Tooltip(h refract.Hit) TooltipContent
}

// TooltipFunc adapts a plain function to [Tooltipper].
type TooltipFunc func(h refract.Hit) TooltipContent

// Tooltip calls f.
func (f TooltipFunc) Tooltip(h refract.Hit) TooltipContent { return f(h) }

var _ Tooltipper = TooltipFunc(nil)

// tooltip is what a hover says, drawn over the chart rather than into it.
//
// It is its own raster rather than a canvas.Text, and the reason is glyph
// coverage. Fyne shapes text through the theme's font and, for a character
// that font has no glyph for, draws U+FFFD — see internal/painter/font.go,
// which substitutes the replacement character for a .notdef glyph. A theme
// carrying its own font resource has no fallback at all. So the very
// characters a chart reaches for, ≤ ≥ ∞ ±, came out as "�" in the one place a
// chart puts numbers in front of a reader.
//
// Drawing the tooltip through the same rasterizer that draws the chart's own
// labels fixes that — the box says what the axis beside it says, in the same
// typeface, with the same coverage — and it brings multi-line labels with it,
// which a canvas.Text cannot draw.
//
// It stays outside the chart's own frame regardless: a tooltip must not be
// clipped by the plot area, and rasterizing the whole chart on every pointer
// move is what this widget spends its effort avoiding.
type tooltip struct {
	c   *Chart
	img *canvas.Image

	// draw is the surface the tooltip's box is rasterized in, and meas the
	// one-pixel surface its text is measured on. Measuring needs an open
	// backend and sizing the box needs the measurements, so they cannot be
	// the same surface.
	draw    *ggbackend.Surface
	meas    *ggbackend.Surface
	measB   ir.Backend
	measDPR float64

	// fonts is the typeface the box is drawn in — regular, bold, italic — or
	// nil for the rasterizer's own. It follows the chart's: a tooltip in a
	// different face from the axis under it would look like a bug.
	fonts [3][]byte

	// last is what has been rendered, so that a pointer moving along one mark
	// re-renders nothing.
	last  tipKey
	drawn bool
}

// tipKey identifies a rendered tooltip.
type tipKey struct {
	text  string
	style TooltipStyle
	dpr   float64
}

// tooltipPad is the default space between the tooltip's text and its edge, and
// the space between the tooltip and the pointer.
const tooltipPad = 6

// tooltipLineSpacing is the default distance between the baselines of a
// multi-line tooltip, as a multiple of the line height.
const tooltipLineSpacing = 1.25

func newTooltip(c *Chart) *tooltip {
	img := canvas.NewImageFromImage(image.NewNRGBA(image.Rect(0, 0, 1, 1)))
	img.FillMode = canvas.ImageFillStretch
	img.ScaleMode = canvas.ImageScaleFastest
	img.Hide()

	t := &tooltip{c: c, img: img}
	if c.cfg.font {
		if regular, bold, italic, ok := c.themeFonts(); ok {
			t.fonts = [3][]byte{regular, bold, italic}
		}
	}
	return t
}

func (t *tooltip) objects() []fyne.CanvasObject {
	if t == nil {
		return nil
	}
	return []fyne.CanvasObject{t.img}
}

// setFont puts the tooltip in the typeface the chart has just been rebuilt in.
// Pass all three as nil for the rasterizer's own fonts.
func (t *tooltip) setFont(regular, bold, italic []byte) {
	if t == nil {
		return
	}
	t.fonts = [3][]byte{regular, bold, italic}
	t.release()
}

// close releases the tooltip's surfaces. The chart is closing.
func (t *tooltip) close() {
	if t == nil {
		return
	}
	t.release()
}

// release drops both surfaces and the render cache, so that the next tooltip
// builds them again.
func (t *tooltip) release() {
	if t.draw != nil {
		_ = t.draw.Close()
		t.draw = nil
	}
	if t.meas != nil {
		_ = t.meas.Close()
		t.meas = nil
	}
	t.measB, t.measDPR = nil, 0
	t.last, t.drawn = tipKey{}, false
}

// show places the tooltip for a hover, or hides it when the hover found
// nothing.
func (t *tooltip) show(ev refract.Event) {
	// A plot keeps its handlers for good, so a chart that has been closed —
	// and a second chart on the same plot — can still be told about a hover
	// that is not its own. A chart with nothing open has nothing to say.
	if t == nil || !t.c.cfg.tooltip || t.c.live == nil {
		return
	}
	if !ev.Found {
		t.hide()
		return
	}
	content := t.c.tipContent(ev.Hit)
	if content.Text == "" {
		t.hide()
		return
	}

	dpr := t.c.dpr
	if dpr <= 0 {
		dpr = 1
	}
	key := tipKey{text: content.Text, style: content.Style, dpr: dpr}
	if !t.drawn || key != t.last {
		if err := t.render(content, dpr); err != nil {
			t.c.renderr = err
			t.hide()
			return
		}
		t.last, t.drawn = key, true
	}

	at := t.place(fyne.NewPos(ev.Point.X, ev.Point.Y), t.img.Size())
	t.img.Move(at)
	if !t.img.Visible() {
		t.img.Show()
	}
	canvas.Refresh(t.img)
}

// hide takes the tooltip off the chart.
func (t *tooltip) hide() {
	if t == nil || !t.img.Visible() {
		return
	}
	t.img.Hide()
	canvas.Refresh(t.img)
}

// render draws the tooltip's box and label into its own surface, and hands the
// pixels to the canvas object.
func (t *tooltip) render(content TooltipContent, dpr float64) error {
	st := content.Style
	lines := strings.Split(content.Text, "\n")
	font := ir.FontRef{Size: float64(st.FontSize), Italic: st.Italic}
	if st.Bold {
		font.Weight = 700
	}

	m, err := t.measurer(dpr)
	if err != nil {
		return err
	}
	var width, ascent, descent float32
	for _, line := range lines {
		mt := m.Measure(ir.TextRun{Text: line, Font: font})
		width = max32(width, mt.Advance)
		ascent = max32(ascent, mt.Ascent)
		descent = max32(descent, mt.Descent)
	}
	if width <= 0 || ascent+descent <= 0 {
		return errors.New("fyne-refract/chart: the tooltip's text measured as nothing")
	}

	spacing := st.LineSpacing
	if spacing <= 0 {
		spacing = tooltipLineSpacing
	}
	pad := max32(st.Padding, 0)
	border := st.BorderWidth
	if border < 0 {
		border = 0
	}
	lineH := ascent + descent
	step := lineH * spacing
	w := int(math.Ceil(float64(width + 2*pad)))
	h := int(math.Ceil(float64(lineH + step*float32(len(lines)-1) + 2*pad)))

	b, err := t.open(w, h, dpr)
	if err != nil {
		return err
	}

	// The box, inset by half the outline so that the stroke lands inside the
	// surface rather than half outside it.
	box := &ir.Path{}
	roundRect(box, ir.R(border/2, border/2, float32(w)-border/2, float32(h)-border/2), st.Corner)
	if fill := nrgba(st.Background); fill.A != 0 {
		b.FillPath(box, ir.Solid(fill), ir.NonZero)
	}
	if edge := nrgba(st.Border); border > 0 && edge.A != 0 {
		b.StrokePath(box, ir.Stroke{Color: edge, Width: border, Join: ir.JoinRound})
	}

	label := nrgba(st.Text)
	y := pad + ascent
	for _, line := range lines {
		b.Text(ir.TextRun{
			Text:  line,
			Font:  font,
			At:    ir.Point{X: pad, Y: y},
			H:     ir.AlignStart,
			V:     ir.AlignBaseline,
			Color: label,
		})
		y += step
	}
	if err := b.Flush(); err != nil {
		return err
	}

	img := t.draw.Image()
	if img == nil {
		return errors.New("fyne-refract/chart: the tooltip drew no pixels")
	}
	// A copy, because the surface's buffer belongs to the surface: the next
	// tooltip opens it again, and Fyne's painter may still be reading this
	// frame when it does.
	t.img.Image = clone(img)
	t.img.Resize(fyne.NewSize(float32(w), float32(h)))
	t.img.Refresh()
	return nil
}

// measurer is a backend the tooltip's text can be measured on, in the fonts it
// will be drawn in. It is one pixel: nothing is drawn into it.
func (t *tooltip) measurer(dpr float64) (ir.Backend, error) {
	if t.measB != nil && t.measDPR == dpr {
		return t.measB, nil
	}
	if t.meas == nil {
		t.meas = t.surface()
	}
	b, err := t.meas.Open(1, 1, dpr)
	if err != nil {
		return nil, err
	}
	t.measB, t.measDPR = b, dpr
	return b, nil
}

// open prepares the drawing surface for a box of the given size.
//
// It opens it every time rather than only when the size changed, because
// opening is what clears it: a rasterizer has no "erase", and a tooltip whose
// text got shorter would otherwise keep the tail of the last one.
func (t *tooltip) open(w, h int, dpr float64) (ir.Backend, error) {
	if t.draw == nil {
		t.draw = t.surface()
	}
	return t.draw.Open(w, h, dpr)
}

// surface returns a rasterizer in the tooltip's fonts.
func (t *tooltip) surface() *ggbackend.Surface {
	var opts []ggbackend.Option
	if len(t.fonts[0]) > 0 {
		opts = append(opts, ggbackend.WithFont(t.fonts[0], t.fonts[1], t.fonts[2]))
	}
	return ggbackend.NewSurface(opts...)
}

// place puts the tooltip beside the pointer, and on the other side of it
// rather than off the edge when there is no room.
func (t *tooltip) place(at fyne.Position, box fyne.Size) fyne.Position {
	size := t.c.Size()
	x, y := at.X+tooltipPad, at.Y+tooltipPad
	if x+box.Width > size.Width {
		x = at.X - tooltipPad - box.Width
	}
	if y+box.Height > size.Height {
		y = at.Y - tooltipPad - box.Height
	}
	return fyne.NewPos(max32(x, 0), max32(y, 0))
}

// tipContent is what the chart says about a hit, with everything its options
// and the Fyne theme leave to it filled in.
func (c *Chart) tipContent(h refract.Hit) TooltipContent {
	var out TooltipContent
	if c.cfg.content != nil {
		out = c.cfg.content(h)
	} else if c.cfg.format != nil {
		out.Text = c.cfg.format(h)
	}
	out.Style = out.Style.over(c.cfg.style.over(c.themeTooltipStyle()))
	return out
}

// themeTooltipStyle is what a tooltip looks like before anyone says otherwise:
// an overlay in the theme's colours, at its caption size.
func (c *Chart) themeTooltipStyle() TooltipStyle {
	th := c.Theme()
	var v fyne.ThemeVariant
	if app := fyne.CurrentApp(); app != nil {
		v = app.Settings().ThemeVariant()
	}
	return TooltipStyle{
		Text:        th.Color(fynetheme.ColorNameForeground, v),
		Background:  th.Color(fynetheme.ColorNameOverlayBackground, v),
		Border:      th.Color(fynetheme.ColorNameSeparator, v),
		BorderWidth: 1,
		Corner:      th.Size(fynetheme.SizeNameSelectionRadius),
		Padding:     tooltipPad,
		FontSize:    th.Size(fynetheme.SizeNameCaptionText),
		LineSpacing: tooltipLineSpacing,
	}
}

// kappa is the cubic approximation of a quarter circle.
const kappa = 0.5522847498307936

// roundRect appends a rectangle with rounded corners to p.
func roundRect(p *ir.Path, r ir.Rect, radius float32) {
	if radius > 0 {
		radius = min32(radius, min32(r.Dx(), r.Dy())/2)
	}
	if radius <= 0 {
		p.Rect(r)
		return
	}
	k := radius * kappa
	x0, y0, x1, y1 := r.Min.X, r.Min.Y, r.Max.X, r.Max.Y
	p.MoveTo(x0+radius, y0)
	p.LineTo(x1-radius, y0)
	p.CubicTo(x1-radius+k, y0, x1, y0+radius-k, x1, y0+radius)
	p.LineTo(x1, y1-radius)
	p.CubicTo(x1, y1-radius+k, x1-radius+k, y1, x1-radius, y1)
	p.LineTo(x0+radius, y1)
	p.CubicTo(x0+radius-k, y1, x0, y1-radius+k, x0, y1-radius)
	p.LineTo(x0, y0+radius)
	p.CubicTo(x0, y0+radius-k, x0+radius-k, y0, x0+radius, y0)
	p.Close()
}

// clone copies a frame out of a surface's buffer.
func clone(src image.Image) image.Image {
	b := src.Bounds()
	dst := image.NewNRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	draw.Draw(dst, dst.Bounds(), src, b.Min, draw.Src)
	return dst
}

// nrgba is a theme colour in the eight-bit non-premultiplied channels the IR
// speaks in. A nil colour is transparent, which draws nothing.
func nrgba(c color.Color) ir.Color {
	if c == nil {
		return ir.Color{}
	}
	if n, ok := c.(color.NRGBA); ok {
		return n
	}
	n, _ := color.NRGBAModel.Convert(c).(color.NRGBA)
	return n
}

func max32(a, b float32) float32 {
	if a > b {
		return a
	}
	return b
}

func min32(a, b float32) float32 {
	if a < b {
		return a
	}
	return b
}
