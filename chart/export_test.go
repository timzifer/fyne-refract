package chart

import "github.com/timzifer/refract"

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
func TipContent(c *Chart, h refract.Hit) TooltipContent { return c.tipContent(h) }
