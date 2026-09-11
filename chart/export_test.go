package chart

import "github.com/timzifer/figure"

// TipText is what the chart's tooltip last rendered. A tooltip is chrome and
// has no API of its own, so this is how a test outside the package reads it.
func TipText(c *Chart) string {
	if c.tip == nil {
		return ""
	}
	return c.tip.last.text
}

// TipContent is what the chart would say about a hit, styling included. It is
// the resolution of [TooltipFormat], [TooltipContentFunc], [TooltipWith] and
// [TooltipLook] against the theme, without a hover to trigger it.
func TipContent(c *Chart, h figure.Hit) TooltipContent { return c.tipContent(h) }

// Pointer is the layer that takes the pointer for a chart.
type Pointer = pointer

// PointerOf is where a test hands a chart the events Fyne's driver would,
// without a canvas deciding whether they reach it. It works whether or not the
// chart is [Interactive]; a test of that goes through the canvas.
func PointerOf(c *Chart) *Pointer { return c.ptr }
