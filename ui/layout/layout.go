package layout

import (
	"image"
	"image/color"

	"gioui.org/layout"
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
