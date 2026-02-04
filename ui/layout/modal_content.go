package layout

import (
	"image/color"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"gioui.org/x/component"
	"golang.org/x/exp/shiny/materialdesign/colornames"
	"golang.org/x/exp/shiny/materialdesign/icons"
)

type ModalContent struct {
	*material.Theme
	header  Header
	OnClose func()
	layout.List
}

type Header struct {
	*material.Theme
	closeButton widget.Clickable
	closeIcon   *widget.Icon
}

func (h *Header) CloseButtonClicked(gtx layout.Context) bool {
	return h.closeButton.Clicked(gtx)
}

func NewModalContent(theme *material.Theme, onClose func()) *ModalContent {
	iconClear, _ := widget.NewIcon(icons.ContentClear)
	return &ModalContent{
		Theme:   theme,
		header:  Header{Theme: theme, closeIcon: iconClear},
		OnClose: onClose,
		List:    layout.List{Axis: layout.Vertical},
	}
}

func (m *ModalContent) DrawContent(gtx layout.Context, contentWidget layout.Widget) layout.Dimensions {
	if m.header.CloseButtonClicked(gtx) {
		if m.OnClose != nil {
			m.OnClose()
		}
	}
	mac := op.Record(gtx.Ops)
	d := layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			// expand parent content in horizontal direction
			gtx.Constraints.Min.X = gtx.Constraints.Max.X
			margins := layout.Inset{Top: unit.Dp(16), Bottom: unit.Dp(16), Right: unit.Dp(8), Left: unit.Dp(8)}
			return margins.Layout(gtx, m.header.Layout)
		}),
		layout.Rigid(layout.Hr{Height: unit.Dp(1), Color: color.NRGBA(colornames.Grey300)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return m.List.Layout(gtx, 1, func(gtx layout.Context, index int) layout.Dimensions {
				return contentWidget(gtx)
			})
		}),
	)
	call := mac.Stop()
	component.Rect{Color: m.Theme.Bg, Size: d.Size, Radii: gtx.Dp(8)}.Layout(gtx)
	call.Add(gtx.Ops)
	return d
}

func (h *Header) Layout(gtx layout.Context) layout.Dimensions {
	return layout.Flex{Spacing: layout.SpaceBetween, Alignment: layout.Middle}.Layout(gtx,
		layout.Rigid(layout.Spacer{Width: unit.Dp(24)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			bd := material.Body1(h.Theme, "Settings")
			bd.TextSize = unit.Sp(18)
			bd.Font.Weight = font.ExtraBold
			bd.Color = h.Theme.ContrastBg
			return bd.Layout(gtx)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			btn := material.IconButtonStyle{
				Icon:        h.closeIcon,
				Button:      &h.closeButton,
				Description: "close button",
			}
			btn.Inset = layout.UniformInset(unit.Dp(4))
			btn.Size = unit.Dp(24)
			btn.Background = h.Theme.Bg
			btn.Color = h.Theme.ContrastBg
			return btn.Layout(gtx)
		}),
	)
}
