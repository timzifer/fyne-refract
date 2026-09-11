// Package gpu turns on figure's GPU tier for a Fyne chart.
//
// Measured on a 900x480 chart, per frame, against the CPU rasterizer:
//
//	                        CPU      GPU
//	a stream sliding left   25 ms    7 ms
//	panning 4000 points     45 ms    5 ms
//
// # What it is
//
// A thin re-export of figure's own tier, and the place this repository keeps
// its measurements of it: the benchmark above, and a parity test comparing the
// two tiers pixel by pixel.
//
// It carried more than that for a while. wgpu's hardware backends register
// from their own init and nothing in the chain imported one, so opting in
// enumerated no adapters — and gg installs its GPU coverage filler whether or
// not a device answered, so a chart came out as its text with every path
// silently missing. figure's tier now imports a HAL backend itself and proves
// the accelerator draws before keeping it, so [Enabled] means what it says and
// a machine without a device falls back to the CPU as it always promised.
//
// # What it does
//
// Importing it registers gg's GPU accelerator and its tile-based coverage
// filler, which every rasterizer made afterwards uses. Nothing has to be
// passed anywhere:
//
//	import (
//	    "github.com/timzifer/fyne_figure/chart"
//	    _ "github.com/timzifer/fyne_figure/gpu" // opt into the GPU tier
//	)
//
// The import has to happen before the first chart is drawn, which a blank
// import in the program's main package does.
//
// # What it is for
//
// Interaction over a lot of data: a chart panned and zoomed at a size where
// the CPU rasterizer's scanline pass is the frame budget. That is the common
// case for a chart in a window — see the "What a frame costs" section of the
// package next door for what one costs without it.
//
// # Why a module of its own
//
// Because the import is the opt-in, and an opt-in that arrives with a
// dependency nobody asked for is not one. The tier pulls wgpu, naga and a
// foreign-function layer into the build, wants a working Vulkan, Metal or DX12
// at run time, and does not build for js/wasm at all. A chart widget must not
// cost any of that to someone who did not ask for it.
//
// # When there is no GPU
//
// Registration fails quietly and rendering falls back to the CPU: a chart
// still draws on a machine with no usable device, which is the only acceptable
// behaviour. [Enabled] reports which way it went.
//
// It is opt-in beta, which is figure's own position for it and not a
// temporary caveat. The two tiers do not share a coverage filler, so they do
// not agree pixel for pixel: the difference is the antialiasing on the edge of
// everything drawn, about one part in 255 on average. The parity test in this
// package measures it, behind the tierparity build tag.
package gpu

import (
	// The blank import is the whole mechanism: figure's tier registers gg's
	// accelerator from its init, and every rasterizer made after that uses it.
	figuregpu "github.com/timzifer/figure/backend/gg/gpu"
)

// Enabled reports whether the GPU tier took.
//
// Importing this package asks for the GPU; a machine with no Vulkan, Metal or
// DX12 — a container, a VM, a CI runner — says no, and rendering falls back to
// the CPU without a word. This is how a program that would rather know can
// find out, to say so in a status bar or a log line.
//
// It is an answer about drawing rather than about registration: figure's tier
// proves the accelerator puts ink in a buffer before it keeps it, and gives it
// back otherwise.
func Enabled() bool { return figuregpu.Enabled() }

// Close releases the GPU device and everything held on it, after which
// rendering falls back to the CPU rasterizer. It is what a program defers from
// main; calling it twice, or with no GPU registered, does nothing.
func Close() { figuregpu.Close() }
