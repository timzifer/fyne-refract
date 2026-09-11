package main

import (
	"fmt"
	"math"
	"slices"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
	"github.com/timzifer/figure/geom"
	"github.com/timzifer/figure/palette"
	"github.com/timzifer/figure/scale"
	"github.com/timzifer/figure/three"
	fynefigure "github.com/timzifer/fyne-figure"
	"github.com/timzifer/fyne-figure/orbit"
)

// camerasTab is the 3D tab's four cameras as four widgets, with the glue that
// made them one figure written out here instead of inside the widget.
//
// One orbit widget with four views shares a selection by construction and turns
// its views apart unless orbit.Together says otherwise. Four widgets share
// nothing: each is a plot of its own, and everything that makes them one
// figure is a handler in this file — a pick ringed in all four, a turn that
// optionally turns all four, and the level-of-detail trade that keeps that
// affordable. That is the arrangement a program wants when the views live in
// different parts of a window, or when it wants to decide per view what links.
//
// # Keeping up
//
// With "turn together" on, a drag in one widget redraws all four on every
// frame, one after another on Fyne's goroutine. The one under the pointer is
// the one the reader is watching, so the others can be drawn at half
// resolution while the gesture lasts and sharpened when it ends:
// orbit.Detail(0.5) says how coarse, Chart.OnGesture says when, and
// Chart.SetCoarse does it. The status line shows the time a frame takes, so
// the difference can be read off rather than guessed at.
func camerasTab() fyne.CanvasObject {
	labels := []string{"three-quarter", "plan", "front", "side"}
	cams := []three.Camera{
		three.Home(),
		three.LookAt(three.Elevation(1.45)),
		three.LookAt(three.Azimuth(0), three.Elevation(0.02)),
		three.LookAt(three.Azimuth(-math.Pi/2), three.Elevation(0.02)),
	}

	status := widget.NewLabel("")
	g := &glue{labels: labels, homes: cams, last: slices.Clone(cams), turning: -1, status: status}
	for i, label := range labels {
		// A scene each: every plot trains its own scales, and nothing about one
		// widget's drawing reaches into another's.
		p := three.New(three.Size(440, 240), three.Title(label)).
			Scene(saddleScene()).
			Add(three.View{Camera: cams[i]})
		g.charts = append(g.charts, orbit.New(p,
			orbit.Interactive(true),
			orbit.Select(true),
			orbit.Detail(0.5),
		))
	}
	for i, c := range g.charts {
		c.OnCamera(func(_ int, cam three.Camera) { g.turned(i, cam) })
		c.OnSelect(func(sel fynefigure.Selection) { g.picked(i, sel) })
		c.OnGesture(func(active bool) { g.gesture(i, active) })
	}
	g.idle()

	together := widget.NewCheck("Turn together", func(on bool) { g.together = on })
	coarse := widget.NewCheck("Draw the others at half resolution while one turns", func(on bool) { g.coarse = on })
	together.SetChecked(true)
	coarse.SetChecked(true)
	home := widget.NewButton("Home", g.home)
	clear := widget.NewButton("Clear selection", func() {
		for _, c := range g.charts {
			c.SetSelection(nil)
		}
		g.idle()
	})

	grid := container.NewGridWithColumns(2)
	for _, c := range g.charts {
		grid.Add(c)
	}
	controls := container.NewHBox(home, clear, together, coarse)
	return container.NewBorder(nil, container.NewVBox(status, controls), nil, nil, grid)
}

// glue is everything that makes four widgets one figure. Every method runs on
// Fyne's goroutine, inside a handler of the chart named by from, so it calls
// into the other three and never back into that one.
type glue struct {
	charts []*orbit.Chart
	labels []string
	homes  []three.Camera

	// last is where each widget's camera was when this last heard, which is
	// what a turn is measured from.
	last []three.Camera

	together, coarse bool

	// homing says the cameras are being put back from here, so that the
	// reports that causes are not turns to pass on.
	homing bool

	// turning is the widget a gesture is in, or -1; frame and cost time it.
	turning int
	frame   time.Time
	cost    time.Duration

	status *widget.Label
}

// turned passes one widget's turn on to the other three, if they turn
// together. A turn is passed on as an amount rather than as a camera, for the
// reason orbit.Together gives: a plan, a front and a side turned together stay
// a plan, a front and a side, turned.
func (g *glue) turned(from int, cam three.Camera) {
	was := g.last[from]
	g.last[from] = cam
	if !g.homing && g.together {
		dAz, dEl := cam.Azimuth()-was.Azimuth(), cam.Elevation()-was.Elevation()
		by := cam.Zoom() / was.Zoom()
		for i, c := range g.charts {
			if i == from {
				continue
			}
			// SetCamera reports nothing back, so this does not come round again.
			g.last[i] = three.Orbit(three.Dolly(g.last[i], by), dAz, dEl)
			_ = c.SetCamera(0, g.last[i])
		}
	}
	g.timed(from)
}

// picked rings one widget's pick in the other three. SetSelection reports
// nothing back, so it does not loop.
func (g *glue) picked(from int, sel fynefigure.Selection) {
	for i, c := range g.charts {
		if i != from {
			c.SetSelection(sel)
		}
	}
	if len(sel) == 0 {
		g.idle()
		return
	}
	g.status.SetText(fmt.Sprintf("row %d picked in the %s widget, and marked in all %d",
		sel[0].Row, g.labels[from], len(g.charts)))
}

// gesture coarsens the other three while one is turned, and sharpens them when
// it lets go.
func (g *glue) gesture(from int, active bool) {
	for i, c := range g.charts {
		if i != from {
			c.SetCoarse(active && g.coarse)
		}
	}
	if active {
		g.turning, g.frame, g.cost = from, time.Time{}, 0
		return
	}
	g.turning = -1
}

// timed measures the time between two frames of a gesture — every widget that
// was redrawn for it, not only the one under the pointer — and shows it.
func (g *glue) timed(from int) {
	if from != g.turning {
		return
	}
	now := time.Now()
	if !g.frame.IsZero() {
		d := now.Sub(g.frame)
		if g.cost == 0 {
			g.cost = d
		} else {
			// Smoothed, so that the number can be read while it changes.
			g.cost = (g.cost*7 + d) / 8
		}
		g.status.SetText(fmt.Sprintf("turning the %s widget: %.0f ms a frame",
			g.labels[from], float64(g.cost)/float64(time.Millisecond)))
	}
	g.frame = now
}

// home puts every widget back where its author pointed it.
func (g *glue) home() {
	g.homing = true
	for _, c := range g.charts {
		_ = c.Home()
	}
	g.homing = false
	g.last = slices.Clone(g.homes)
}

func (g *glue) idle() {
	g.status.SetText("Four widgets, one figure. Drag one to turn it, click a point to mark it in all four.")
}

// saddleScene is the 3D tab's surface as a scene of its own.
func saddleScene() *three.Scene {
	return three.NewScene(three.XTitle("x"), three.YTitle("y"), three.ZTitle("response")).
		Z(scale.Linear(scale.Nice())).
		Add(three.Surface(saddle(), geom.X("x"), geom.Y("y"), geom.Z("z"),
			geom.Fill(palette.SkyBlue), geom.Label("response")))
}
