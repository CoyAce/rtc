package ui

import (
	"image"
	"rtc/assets"

	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"golang.org/x/exp/shiny/materialdesign/icons"
)

type Avatar struct {
	Size          int
	Editable      bool
	EditButton    IconButton
	point         image.Point
	Image         image.Image
	selectedImage chan image.Image
	widget.Clickable
	*material.Theme
}

func (v *Avatar) Layout(gtx layout.Context) layout.Dimensions {
	if v.Size == 0 {
		v.Size = 48
	}
	if v.selectedImage == nil {
		v.selectedImage = make(chan image.Image)
	}
	v.point = image.Point{X: gtx.Dp(unit.Dp(v.Size)), Y: gtx.Dp(unit.Dp(v.Size))}
	if v.Image == nil {
		v.Image = assets.AppIconImage
	}
	if v.Editable && v.Clicked(gtx) {
		go func() {
			file, err := picker.ChooseFile(".jpg", ".png")
			if err != nil {
				return
			}
			var img, _, _ = image.Decode(file)
			v.selectedImage <- img
		}()
	}
	if v.Editable {
		select {
		case img, ok := <-v.selectedImage:
			if ok {
				v.Image = img
			}
		default:
		}
	}
	gtx.Constraints.Min, gtx.Constraints.Max = v.point, v.point
	return layout.Stack{Alignment: layout.Center}.Layout(gtx,
		layout.Stacked(func(gtx layout.Context) layout.Dimensions {
			return v.Clickable.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				imgOps := paint.NewImageOp(v.Image)
				imgWidget := widget.Image{Src: imgOps, Fit: widget.Fill, Position: layout.Center, Scale: 0}
				defer clip.UniformRRect(image.Rectangle{
					Max: image.Point{
						X: gtx.Constraints.Max.X,
						Y: gtx.Constraints.Max.Y,
					},
				}, gtx.Constraints.Max.X/2).Push(gtx.Ops).Pop()
				return imgWidget.Layout(gtx)
			})
		}),
		layout.Stacked(func(gtx layout.Context) layout.Dimensions {
			if !v.Editable || !v.Hovered() {
				return layout.Dimensions{}
			}
			gtx.Constraints.Max.X = int(float64(v.point.X) * 0.40)
			gtx.Constraints.Max.Y = int(float64(v.point.Y) * 0.40)
			gtx.Constraints.Min = gtx.Constraints.Max
			iconClr := v.Theme.ContrastFg
			icon, _ := widget.NewIcon(icons.ImageEdit)
			return icon.Layout(gtx, iconClr)
		}),
	)
}
