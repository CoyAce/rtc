package ui

import (
	"image"
	"math"
	"time"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"gioui.org/x/component"
	"golang.org/x/exp/shiny/materialdesign/icons"
)

func Layout(gtx C, msg string, theme *material.Theme) (d D) {
	if msg == "" {
		return d
	}

	isMe := true
	margins := layout.Inset{Top: unit.Dp(24), Bottom: unit.Dp(0), Right: unit.Dp(8)}
	return margins.Layout(gtx, func(gtx C) D {
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		flex := layout.Flex{Axis: layout.Vertical}
		if isMe {
			flex.Alignment = layout.End
		} else {
			flex.Alignment = layout.Start
		}
		return flex.Layout(gtx,
			layout.Rigid(func(gtx C) D {
				timeVal := time.Now()
				txtMsg := timeVal.Local().Format("Mon, Jan 2, 3:04 PM")
				label := material.Label(theme, theme.TextSize*0.70, txtMsg)
				label.Color = theme.ContrastBg
				label.Color.A = uint8(int(math.Abs(float64(label.Color.A)-50)) % 256)
				label.Font.Weight = font.Bold
				label.Font.Style = font.Italic
				margins = layout.Inset{Bottom: unit.Dp(8.0)}
				d := margins.Layout(gtx, func(gtx C) D {
					flex := layout.Flex{}
					if isMe {
						flex.Spacing = layout.SpaceStart
					} else {
						flex.Spacing = layout.SpaceEnd
					}
					return flex.Layout(gtx,
						layout.Rigid(func(gtx C) D {
							return component.TruncatingLabelStyle(label).Layout(gtx)
						}))
				})
				return d
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				flex := layout.Flex{Spacing: layout.SpaceEnd, Alignment: layout.Middle}
				if isMe {
					flex.Spacing = layout.SpaceStart
				}
				return flex.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						if isMe {
							icon, _ := widget.NewIcon(icons.AlertErrorOutline)
							iconColor := theme.ContrastBg
							icon, _ = widget.NewIcon(icons.ActionDone)
							//icon, _ = widget.NewIcon(icons.ActionDoneAll)
							//switch p.Message.State {
							//case chat.MessageStateReceived:
							//	icon, _ = widget.NewIcon(icons.ActionDone)
							//case chat.MessageStateRead:
							//	icon, _ = widget.NewIcon(icons.ActionDoneAll)
							//}
							return icon.Layout(gtx, iconColor)
						}
						return D{}
					}),
					layout.Rigid(func(gtx C) D {
						if msg != "" {
							macro := op.Record(gtx.Ops)
							inset := layout.UniformInset(unit.Dp(12))
							d := inset.Layout(gtx, func(gtx C) D {
								flex := layout.Flex{}
								gtx.Constraints.Min.X = 0
								return flex.Layout(gtx,
									layout.Rigid(func(gtx C) D {
										gtx.Constraints.Max.X = int(float32(gtx.Constraints.Max.X) / 1.5)
										bd := material.Body1(theme, msg)
										return bd.Layout(gtx)
									}))
							})
							call := macro.Stop()
							bgColor := theme.ContrastBg
							bgColor.A = 50
							radius := gtx.Dp(16)
							sE, sW, nW, nE := radius, radius, radius, radius
							if isMe {
								nE = 0
							} else {
								nW = 0
							}
							clipOp := clip.RRect{Rect: image.Rectangle{
								Max: image.Point{X: d.Size.X, Y: d.Size.Y},
							}, SE: sE, SW: sW, NW: nW, NE: nE}.Push(gtx.Ops)
							component.Rect{Color: bgColor, Size: d.Size}.Layout(gtx)
							call.Add(gtx.Ops)
							clipOp.Pop()
							return d
						}
						return D{}
					}),
				)
			}),
		)
	})
}
