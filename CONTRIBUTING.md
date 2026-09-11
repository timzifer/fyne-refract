# Contributing

## Layout

Three modules, one repository.

| Path | Module | Depends on |
|---|---|---|
| `.` | `github.com/timzifer/fyne-figure` | Fyne, figure, figure's raster backend |
| `gpu` | `github.com/timzifer/fyne-figure/gpu` | the above plus figure's GPU tier, and through it wgpu |
| `cmd/demo` | `github.com/timzifer/fyne-figure/cmd/demo` | both of the above |

The split is not cosmetic. A nested module is excluded from its parent's module
graph, so importing the widget cannot pull a GPU stack into a build that never
asked for one. It is the arrangement figure makes for the same tier one level
up.

The demo is a third module for the other side of that: it is the one thing here
that wants both the widget and the tier, and the only way to import a nested
module is from outside the one it is nested in. Its `replace` directives point
at the two directories above it, and nothing imports the demo, so they cost no
one anything.

Inside the main module the split is figure's own, between `backend/window` and
`backend/window/show`: `fynefigure` draws, and `fynefigure/chart` and
`fynefigure/orbit` steer. A backend must not know what a scale or a panel is,
and everything in `chart` is about scales and panels — and everything in
`orbit` about cameras. A change that needs to cross that line is a sign the
seam is in the wrong place — say so rather than routing around it.

`chart` and `orbit` are two widgets rather than one because figure has two
plots: a `figure.Plot` is steered by figure's own `Input`, and a `three.Plot`
has no scales to pan and leaves its loop to the host. What the two share —
reading Fyne's theme — is in `internal/look`.

## Everyday commands

```sh
# what CI runs
gofmt -l .                                   # must print nothing
go vet ./... && (cd gpu && go vet ./... && go vet -tags tierparity ./...)
go build ./... && (cd gpu && go build ./...)
go test ./... && (cd gpu && go test ./...)
go test -race ./...

# the library must not need cgo; only the demo does, because Fyne's desktop
# driver does
CGO_ENABLED=0 go build . ./chart ./orbit

# a chart in a window, by hand — the one thing CI cannot check. The demo is a
# module of its own, because it opts into the GPU tier and that tier is nested
# inside this one: see cmd/demo/go.mod.
(cd cmd/demo && go run .)
```

Developing against a figure checkout next door wants a workspace. It is not
committed, and the `require` directives always name published tags:

```sh
go work init . ./gpu ../figure ../figure/backend/gg ../figure/backend/gg/gpu
```

## The measurements

Several defaults in this repository are numbers rather than opinions, and the
benchmarks are where they come from. Re-run them before changing one.

```sh
go test -run='^$' -bench=Frame -benchtime=25x .        # what a frame costs
go test -run='^$' -bench=Drag  -benchtime=20x ./chart  # what the pacing saves
go test -run='^$' -bench=Frame -benchtime=25x ./gpu    # the GPU tier
```

`fynefigure.DamageBudget`, `chart.Detail`, `chart.FrameInterval` and the
choice to draw whole frames rather than partial ones each have a table in their
doc comment. A change that moves one should move its table too.

The GPU tier has a parity test behind a build tag, because it gives the device
back halfway through and every benchmark after it would then measure the CPU
while saying GPU:

```sh
cd gpu && go test -tags tierparity -run Parity ./...
```

## Tests

They need no display: Fyne's test driver paints in software. What is asserted
is what figure reports — the size the chart was laid out at, the domain after
a zoom, the frames it painted — rather than how it looks. A test that scraped
pixels to check an interaction would be testing the rasterizer.

Where a test does read pixels it is because pixels are the subject: that the
widget draws what the same plot exports, that a followed axis is drawn at all,
that a font has the glyphs a chart uses.

Write the test that would have caught the bug.
`TestAFollowedAxisMovesWithEveryFrameRatherThanInSteps` exists because a
sliding chart's axis once held still for half a tick and then jumped.

## Things worth knowing before changing them

**Drawing happens under `Target.Render`.** The rasterizer is single-goroutine
and Fyne reads its buffer from the painter, which is the drawing goroutine on a
desktop driver and is not under the test one. Resting on the two being the same
is what the lock exists to avoid. `Render` is not reentrant, and `Plot.Live`
and `Live.Close` both reach back into the target — so neither goes inside one.

**Input is paced, and dropping an event loses nothing.** figure pans by the
distance from the last position it was told about, and wheel deltas are summed,
which is exact. Anything added to the gesture path has to keep that true.

**A followed axis is released and un-niced.** Training accumulates and a linear
axis rounds its domain outward; both are right for a still chart and wrong for
a sliding window. See `chart.Follow`.

**Partial repaints are off on purpose.** They cost six times what they save on
the CPU rasterizer. The number is in `fynefigure.DamageBudget`.
