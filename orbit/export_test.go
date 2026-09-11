package orbit

// Pointer is the layer that takes the pointer for a chart.
type Pointer = pointer

// PointerOf is where a test hands a chart the events Fyne's driver would,
// without a canvas deciding whether they reach it. It works whether or not the
// chart is [Interactive]; a test of that goes through the canvas.
func PointerOf(c *Chart) *Pointer { return c.ptr }
