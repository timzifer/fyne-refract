package gpu_test

import (
	ggbackend "github.com/timzifer/refract/backend/gg"
	"github.com/timzifer/refract/ir"
)

// whole is a target that repaints every frame in full, which is what the
// widget does by default: see fynerefract.DamageBudget. It is repeated here
// rather than imported because this module deliberately does not depend on the
// one it sits inside, and because the comparison is worthless if the two
// tables were measured under different rules.
type whole struct{ *ggbackend.Surface }

func (w whole) Open(widthPx, heightPx int, dpr float64) (ir.Backend, error) {
	b, err := w.Surface.Open(widthPx, heightPx, dpr)
	if err != nil {
		return nil, err
	}
	return full{b}, nil
}

type full struct{ ir.Backend }

func (f full) Damage([]ir.Rect) {
	if p, ok := f.Backend.(ir.Partial); ok {
		p.Damage(nil)
	}
}

func (f full) Resize(widthPx, heightPx int, dpr float64) error {
	if r, ok := f.Backend.(ir.Resizer); ok {
		return r.Resize(widthPx, heightPx, dpr)
	}
	return nil
}
