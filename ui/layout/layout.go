package layout

import (
	"image"
	"image/color"

	"gioui.org/f32"
	"gioui.org/layout"
	gio "gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/x/component"
	"golang.org/x/exp/shiny/materialdesign/colornames"
)

type Hr struct {
	Height unit.Dp
}

func (hr Hr) Layout(gtx layout.Context) layout.Dimensions {
	return component.Rect{
		Color: color.NRGBA(colornames.Grey300),
		Size:  image.Point{Y: gtx.Dp(hr.Height), X: gtx.Constraints.Max.X},
		Radii: 0,
	}.Layout(gtx)
}

// Axis is the Horizontal or Vertical direction.
type Axis uint8

const (
	Horizontal Axis = iota
	Vertical
)

// Alignment is the mutual alignment of a list of widgets.
type Alignment uint8

const (
	Start Alignment = iota
	End
	Middle
	Baseline
)

// Convert a point in (x, y) coordinates to (main, cross) coordinates,
// or vice versa. Specifically, Convert((x, y)) returns (x, y) unchanged
// for the horizontal axis, or (y, x) for the vertical axis.
func (a Axis) Convert(pt image.Point) image.Point {
	if a == Horizontal {
		return pt
	}
	return image.Pt(pt.Y, pt.X)
}

// FConvert a point in (x, y) coordinates to (main, cross) coordinates,
// or vice versa. Specifically, FConvert((x, y)) returns (x, y) unchanged
// for the horizontal axis, or (y, x) for the vertical axis.
func (a Axis) FConvert(pt f32.Point) f32.Point {
	if a == Horizontal {
		return pt
	}
	return f32.Pt(pt.Y, pt.X)
}

// mainConstraint returns the min and max main constraints for axis a.
func (a Axis) mainConstraint(cs gio.Constraints) (int, int) {
	if a == Horizontal {
		return cs.Min.X, cs.Max.X
	}
	return cs.Min.Y, cs.Max.Y
}

// crossConstraint returns the min and max cross constraints for axis a.
func (a Axis) crossConstraint(cs gio.Constraints) (int, int) {
	if a == Horizontal {
		return cs.Min.Y, cs.Max.Y
	}
	return cs.Min.X, cs.Max.X
}

// constraints returns the constraints for axis a.
func (a Axis) constraints(mainMin, mainMax, crossMin, crossMax int) gio.Constraints {
	if a == Horizontal {
		return gio.Constraints{Min: image.Pt(mainMin, crossMin), Max: image.Pt(mainMax, crossMax)}
	}
	return gio.Constraints{Min: image.Pt(crossMin, mainMin), Max: image.Pt(crossMax, mainMax)}
}

func (a Axis) String() string {
	switch a {
	case Horizontal:
		return "Horizontal"
	case Vertical:
		return "Vertical"
	default:
		panic("unreachable")
	}
}
