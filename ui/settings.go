package ui

import (
	"image"
	"rtc/core"
	ui "rtc/ui/layout"

	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"gioui.org/x/component"
	"golang.org/x/exp/shiny/materialdesign/icons"
)

type settingsForm struct {
	*material.Theme
	client           *core.Client
	avatar           Avatar
	onSuccess        func(gtx layout.Context)
	nicknameEditor   *component.TextField
	signEditor       *component.TextField
	serverAddrEditor *component.TextField
	submitButton     IconButton
}

func NewSettingsForm(theme *material.Theme, client *core.Client, onSuccess func(gtx layout.Context)) ui.View {
	submitIcon, _ := widget.NewIcon(icons.ActionDone)
	s := &settingsForm{
		Theme:            theme,
		avatar:           Avatar{Size: 64, Editable: true, Theme: theme},
		onSuccess:        onSuccess,
		client:           client,
		nicknameEditor:   &component.TextField{Editor: widget.Editor{}},
		signEditor:       &component.TextField{Editor: widget.Editor{}},
		serverAddrEditor: &component.TextField{Editor: widget.Editor{}},
		submitButton:     IconButton{Theme: theme, Icon: submitIcon, Enabled: true},
	}
	s.submitButton.OnClick = func(gtx layout.Context) {
		client.Nickname = s.nicknameEditor.Text()
		client.Sign = core.Sign(s.signEditor.Text())
		client.ServerAddr = s.serverAddrEditor.Text()
		client.SendSign()
		client.Store()
		s.onSuccess(gtx)
	}
	return s
}

func (s *settingsForm) Layout(gtx layout.Context) layout.Dimensions {
	s.processClick(gtx)
	if len(s.nicknameEditor.Text()) == 0 {
		s.nicknameEditor.SetText(s.client.Nickname)
	}
	if len(s.signEditor.Text()) == 0 {
		s.signEditor.SetText(string(s.client.Sign))
	}
	if len(s.serverAddrEditor.Text()) == 0 {
		s.serverAddrEditor.SetText(s.client.ServerAddr)
	}
	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	dimensions := layout.Flex{Spacing: layout.SpaceSides}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical, Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(layout.Spacer{Height: unit.Dp(25)}.Layout),
				layout.Rigid(s.avatar.Layout),
				layout.Rigid(layout.Spacer{Height: unit.Dp(15)}.Layout),
				layout.Rigid(s.drawInputArea("Nickname:", func(gtx layout.Context) layout.Dimensions {
					return s.nicknameEditor.Layout(gtx, s.Theme, "")
				})),
				layout.Rigid(layout.Spacer{Height: unit.Dp(15)}.Layout),
				layout.Rigid(s.drawInputArea("Chat Sign:", func(gtx layout.Context) layout.Dimensions {
					return s.signEditor.Layout(gtx, s.Theme, "")
				})),
				layout.Rigid(layout.Spacer{Height: unit.Dp(15)}.Layout),
				layout.Rigid(s.drawInputArea("Server Addr:", func(gtx layout.Context) layout.Dimensions {
					return s.serverAddrEditor.Layout(gtx, s.Theme, "")
				})),
				layout.Rigid(layout.Spacer{Height: unit.Dp(25)}.Layout),
				layout.Rigid(s.submitButton.Layout),
				layout.Rigid(layout.Spacer{Height: unit.Dp(30)}.Layout),
			)
		}),
	)
	defer clip.Rect(image.Rectangle{Max: dimensions.Size}).Push(gtx.Ops).Pop()
	defer pointer.PassOp{}.Push(gtx.Ops).Pop()
	event.Op(gtx.Ops, s)
	return dimensions
}

func (s *settingsForm) processClick(gtx layout.Context) {
	for {
		_, ok := gtx.Event(
			pointer.Filter{
				Target: s,
				Kinds:  pointer.Press,
			},
		)
		if !ok {
			break
		}
		// get focus from editor
		gtx.Execute(key.FocusCmd{})
	}
}

func (s *settingsForm) drawInputArea(label string, widget layout.Widget) func(gtx layout.Context) layout.Dimensions {
	return func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
			layout.Flexed(0.4, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Spacing: layout.SpaceStart}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return material.Label(s.Theme, s.TextSize, label).Layout(gtx)
					}),
				)
			}),
			layout.Rigid(layout.Spacer{Width: unit.Dp(8)}.Layout),
			layout.Flexed(0.6, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Spacing: layout.SpaceEnd}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						gtx.Constraints.Max.X = gtx.Dp(unit.Dp(175))
						return widget(gtx)
					}),
				)
			}))
	}
}
