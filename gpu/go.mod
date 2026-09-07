module github.com/timzifer/fyne-refract/gpu

go 1.25.0

// The GPU tier is a module of its own so that importing the widget cannot pull
// a GPU stack in by accident: a nested module is excluded from its parent's
// module graph, so github.com/timzifer/fyne-refract keeps its dependencies to
// Fyne, refract and refract's raster backend. It is the arrangement refract
// makes for the same tier one level up — see its docs/adr/0022.
//
// It deliberately does not require the module it sits inside. It has nothing to
// say to it — the tier is switched on inside gg, which is what actually draws —
// and a nested module requiring its own parent would need the parent tagged
// before the child could build.

require (
	github.com/timzifer/refract v1.0.0
	github.com/timzifer/refract/backend/gg v1.0.2
	github.com/timzifer/refract/backend/gg/gpu v0.1.2
)

require (
	github.com/go-webgpu/goffi v0.6.3 // indirect
	github.com/go-webgpu/webgpu v0.5.5 // indirect
	github.com/gogpu/gg v0.52.5 // indirect
	github.com/gogpu/gpucontext v0.28.0 // indirect
	github.com/gogpu/gputypes v0.5.2 // indirect
	github.com/gogpu/naga v0.18.0 // indirect
	github.com/gogpu/wgpu v0.31.6 // indirect
	golang.org/x/image v0.44.0 // indirect
	golang.org/x/sys v0.47.0 // indirect
	golang.org/x/text v0.40.0 // indirect
)
