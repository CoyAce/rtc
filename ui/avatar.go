package ui

import (
	"image"
	"rtc/assets"

	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/widget"
	"gioui.org/widget/material"
)

type Avatar struct {
	Size  image.Point
	Image image.Image
	widget.Clickable
	*material.Theme
}

func (v *Avatar) Layout(gtx layout.Context) layout.Dimensions {
	if v.Size == (image.Point{}) {
		v.Size = image.Point{X: gtx.Dp(48), Y: gtx.Dp(48)}
	}
	gtx.Constraints.Min, gtx.Constraints.Max = v.Size, v.Size
	var imgWidget widget.Image
	if v.Image == nil {
		v.Image = assets.AppIconImage
		imgOps := paint.NewImageOp(v.Image)
		imgWidget = widget.Image{Src: imgOps, Fit: widget.Fill, Position: layout.Center, Scale: 0}
	} else {
		imgOps := paint.NewImageOp(v.Image)
		imgWidget = widget.Image{Src: imgOps, Fit: widget.Fill, Position: layout.Center, Scale: 0}
	}
	defer clip.UniformRRect(image.Rectangle{
		Max: image.Point{
			X: gtx.Constraints.Max.X,
			Y: gtx.Constraints.Max.Y,
		},
	}, gtx.Constraints.Max.X/2).Push(gtx.Ops).Pop()
	return imgWidget.Layout(gtx)
}
