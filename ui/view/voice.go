package view

import (
	"image"
	"rtc/assets/fonts"

	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
)

type VoiceRecorder struct {
	InteractiveSpan
	ExpandButton
}

func (v *VoiceRecorder) Layout(gtx layout.Context) layout.Dimensions {
	margins := layout.Inset{Top: unit.Dp(8.0), Left: unit.Dp(8.0), Right: unit.Dp(8), Bottom: unit.Dp(15)}
	return margins.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{
			Axis:      layout.Horizontal,
			Spacing:   layout.SpaceBetween,
			Alignment: layout.Middle,
		}.Layout(gtx,
			// voice input
			layout.Flexed(1.0, func(gtx layout.Context) layout.Dimensions {
				gtx.Constraints.Max.Y = gtx.Dp(42)
				v.InteractiveSpan.Layout(gtx)
				bgColor := fonts.DefaultTheme.ContrastBg
				defer clip.UniformRRect(image.Rectangle{Max: gtx.Constraints.Max}, gtx.Dp(4)).Push(gtx.Ops).Pop()
				paint.Fill(gtx.Ops, bgColor)
				return layout.Dimensions{Size: gtx.Constraints.Max}
			}),
			// expand button
			layout.Rigid(v.ExpandButton.Layout),
		)
	})
}
