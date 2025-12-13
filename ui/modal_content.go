package ui

import (
	ui "rtc/ui/layout"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"gioui.org/x/component"
	"golang.org/x/exp/shiny/materialdesign/icons"
)

type ModalContent struct {
	btnClose  widget.Clickable
	iconClose *widget.Icon
	OnClose   func()
	layout.List
}

func NewModalContent(onCloseClick func()) *ModalContent {
	iconClear, _ := widget.NewIcon(icons.ContentClear)
	return &ModalContent{
		iconClose: iconClear,
		OnClose:   onCloseClick,
		List:      layout.List{Axis: layout.Vertical},
	}
}

func (m *ModalContent) DrawContent(gtx layout.Context, theme *material.Theme, contentWidget layout.Widget) layout.Dimensions {
	if m.iconClose == nil {
		m.iconClose, _ = widget.NewIcon(icons.ContentClear)
	}
	if m.btnClose.Clicked(gtx) {
		if m.OnClose != nil {
			m.OnClose()
		}
	}
	mac := op.Record(gtx.Ops)
	d := layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min.X = gtx.Constraints.Max.X
			vert := unit.Dp(16)
			horiz := unit.Dp(8)
			inset := layout.Inset{Top: vert, Bottom: vert, Right: horiz, Left: horiz}
			return inset.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Spacing: layout.SpaceBetween, Alignment: layout.Middle}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						btn := material.IconButtonStyle{
							Icon:        m.iconClose,
							Button:      &m.btnClose,
							Description: "close backdrop",
						}
						btn.Inset = layout.UniformInset(unit.Dp(4))
						btn.Size = unit.Dp(24)
						btn.Background = theme.ContrastBg
						btn.Color = theme.ContrastFg
						return btn.Layout(gtx)
					}),
					layout.Rigid(layout.Spacer{Width: unit.Dp(8)}.Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						bd := material.Body1(theme, "Protonet")
						bd.TextSize = unit.Sp(18)
						bd.Font.Weight = font.ExtraBold
						bd.Color = theme.ContrastBg
						return bd.Layout(gtx)
					}),
					layout.Rigid(layout.Spacer{Width: unit.Dp(8)}.Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						btn := material.IconButtonStyle{
							Icon:        m.iconClose,
							Button:      &m.btnClose,
							Description: "close backdrop",
						}
						btn.Inset = layout.UniformInset(unit.Dp(4))
						btn.Size = unit.Dp(24)
						btn.Background = theme.ContrastBg
						btn.Color = theme.ContrastFg
						return btn.Layout(gtx)
					}),
				)
			})
		}),
		layout.Rigid(ui.Hr{Height: unit.Dp(1)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return m.List.Layout(gtx, 1, func(gtx layout.Context, index int) layout.Dimensions {
				return contentWidget(gtx)
			})
		}),
	)
	call := mac.Stop()
	component.Rect{Color: theme.Bg, Size: d.Size, Radii: gtx.Dp(8)}.Layout(gtx)
	call.Add(gtx.Ops)
	return d
}
