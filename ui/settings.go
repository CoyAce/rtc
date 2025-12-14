package ui

import (
	ui "rtc/ui/layout"

	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"golang.org/x/exp/shiny/materialdesign/icons"
)

type settingsForm struct {
	*material.Theme
	submitIcon   *widget.Icon
	submitButton widget.Clickable
}

func NewSettingsForm(theme *material.Theme) ui.View {
	submitIcon, _ := widget.NewIcon(icons.ActionDone)
	return &settingsForm{
		Theme:      theme,
		submitIcon: submitIcon,
	}
}

func (s *settingsForm) Layout(gtx layout.Context) layout.Dimensions {
	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	return layout.Flex{Spacing: layout.SpaceSides}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical, Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(layout.Spacer{Height: unit.Dp(25)}.Layout),
				layout.Rigid(avatar.Layout),
				layout.Rigid(layout.Spacer{Height: unit.Dp(15)}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return material.Label(s.Theme, s.TextSize, "Nickname:").Layout(gtx)
				}),
				layout.Rigid(layout.Spacer{Height: unit.Dp(15)}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return material.Label(s.Theme, s.TextSize, "Sign:").Layout(gtx)
				}),
				layout.Rigid(layout.Spacer{Height: unit.Dp(15)}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return material.IconButtonStyle{
						Background: s.Theme.ContrastBg,
						Color:      s.Theme.ContrastFg,
						Icon:       s.submitIcon,
						Size:       unit.Dp(24.0),
						Button:     &s.submitButton,
						Inset:      layout.UniformInset(unit.Dp(9)),
					}.Layout(gtx)
				}),
				layout.Rigid(layout.Spacer{Height: unit.Dp(30)}.Layout),
			)
		}),
	)
}
