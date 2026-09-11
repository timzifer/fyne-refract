// The demo is a module of its own so that it can import the GPU tier.
//
// fyne-figure/gpu is nested in fyne-figure and so excluded from its module
// graph — deliberately, so that a chart widget does not cost a consumer wgpu,
// naga and a foreign-function layer nobody asked for. That leaves no way for a
// package inside fyne-figure to import it: a require in the parent would put
// the GPU stack back in every consumer's graph, which is the arrangement the
// nesting exists to prevent.
//
// A third module resolves it. Nothing imports the demo, so the replace
// directives below cost no one anything, and the widget and the tier stay as
// separate as they were.
module github.com/timzifer/fyne-figure/cmd/demo

go 1.25.0

require (
	fyne.io/fyne/v2 v2.7.3
	github.com/timzifer/figure v0.10.0
	github.com/timzifer/figure/backend/gg v0.9.0
	github.com/timzifer/fyne-figure v0.0.0
	github.com/timzifer/fyne-figure/gpu v0.0.0
)

require (
	fyne.io/systray v1.12.0 // indirect
	github.com/BurntSushi/toml v1.5.0 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/fredbi/uri v1.1.1 // indirect
	github.com/fsnotify/fsnotify v1.9.0 // indirect
	github.com/fyne-io/gl-js v0.2.0 // indirect
	github.com/fyne-io/glfw-js v0.3.0 // indirect
	github.com/fyne-io/image v0.1.1 // indirect
	github.com/fyne-io/oksvg v0.2.0 // indirect
	github.com/go-gl/gl v0.0.0-20231021071112-07e5d0ea2e71 // indirect
	github.com/go-gl/glfw/v3.3/glfw v0.0.0-20240506104042-037f3cc74f2a // indirect
	github.com/go-text/render v0.2.0 // indirect
	github.com/go-text/typesetting v0.3.3 // indirect
	github.com/go-webgpu/goffi v0.6.3 // indirect
	github.com/go-webgpu/webgpu v0.5.5 // indirect
	github.com/godbus/dbus/v5 v5.1.0 // indirect
	github.com/gogpu/gg v0.52.5 // indirect
	github.com/gogpu/gpucontext v0.29.0 // indirect
	github.com/gogpu/gputypes v0.6.0 // indirect
	github.com/gogpu/naga v0.19.0 // indirect
	github.com/gogpu/wgpu v0.32.1 // indirect
	github.com/hack-pad/go-indexeddb v0.3.2 // indirect
	github.com/hack-pad/safejs v0.1.0 // indirect
	github.com/jeandeaual/go-locale v0.0.0-20250612000132-0ef82f21eade // indirect
	github.com/jsummers/gobmp v0.0.0-20230614200233-a9de23ed2e25 // indirect
	github.com/nfnt/resize v0.0.0-20180221191011-83c6a9932646 // indirect
	github.com/nicksnyder/go-i18n/v2 v2.5.1 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/rymdport/portal v0.4.2 // indirect
	github.com/srwiley/oksvg v0.0.0-20221011165216-be6e8873101c // indirect
	github.com/srwiley/rasterx v0.0.0-20220730225603-2ab79fcdd4ef // indirect
	github.com/stretchr/testify v1.11.1 // indirect
	github.com/timzifer/figure/backend/gg/gpu v0.3.0 // indirect
	github.com/yuin/goldmark v1.7.8 // indirect
	golang.org/x/image v0.45.0 // indirect
	golang.org/x/net v0.35.0 // indirect
	golang.org/x/sys v0.47.0 // indirect
	golang.org/x/text v0.41.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace github.com/timzifer/fyne-figure => ../..

replace github.com/timzifer/fyne-figure/gpu => ../../gpu

replace github.com/gogpu/gg => github.com/timzifer/gg v0.52.6-figure.3
