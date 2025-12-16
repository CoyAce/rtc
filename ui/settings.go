package ui

import (
	"image"
	"log"
	"os"
	"rtc/core"
	"time"

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

type SettingsForm struct {
	*material.Theme
	avatar           Avatar
	onSuccess        func(gtx layout.Context)
	nicknameEditor   *component.TextField
	signEditor       *component.TextField
	serverAddrEditor *component.TextField
	submitButton     IconButton
}

func NewSettingsForm(onSuccess func(gtx layout.Context)) *SettingsForm {
	submitIcon, _ := widget.NewIcon(icons.ActionDone)
	s := &SettingsForm{
		Theme:            theme,
		avatar:           Avatar{UUID: client.FullID(), Size: 64, Editable: true, Theme: theme, OnChange: SyncSelectedIcon},
		onSuccess:        onSuccess,
		nicknameEditor:   &component.TextField{Editor: widget.Editor{}},
		signEditor:       &component.TextField{Editor: widget.Editor{}},
		serverAddrEditor: &component.TextField{Editor: widget.Editor{}},
		submitButton:     IconButton{Theme: theme, Icon: submitIcon, Enabled: true},
	}
	s.submitButton.OnClick = func(gtx layout.Context) {
		if s.nicknameEditor.Text() != client.Nickname {
			oldName := core.GetDir(client.FullID())
			client.Nickname = s.nicknameEditor.Text()
			newName := core.GetDir(client.FullID())
			err := os.Rename(oldName, newName)
			if err != nil {
				log.Printf("Failed to rename: %v", err)
			}
		}
		client.Sign = s.signEditor.Text()
		client.ServerAddr = s.serverAddrEditor.Text()
		client.SendSign()
		client.Store()
		s.onSuccess(gtx)
	}
	return s
}

func (s *SettingsForm) Layout(gtx layout.Context) layout.Dimensions {
	s.processClick(gtx)
	if len(s.nicknameEditor.Text()) == 0 {
		s.nicknameEditor.SetText(client.Nickname)
	}
	if len(s.signEditor.Text()) == 0 {
		s.signEditor.SetText(string(client.Sign))
	}
	if len(s.serverAddrEditor.Text()) == 0 {
		s.serverAddrEditor.SetText(client.ServerAddr)
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

func (s *SettingsForm) processClick(gtx layout.Context) {
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

func (s *SettingsForm) drawInputArea(label string, widget layout.Widget) func(gtx layout.Context) layout.Dimensions {
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

func (s *SettingsForm) ShowWithModal(gtx layout.Context) {
	iconStackAnimation.Disappear(gtx.Now)
	modal.Show(ZoomInWithModalContent(s.Layout), nil, component.VisibilityAnimation{
		Duration: time.Millisecond * 250,
		State:    component.Invisible,
		Started:  time.Time{},
	})
}

func ZoomInWithModalContent(widget layout.Widget) func(gtx layout.Context) layout.Dimensions {
	return func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Max.X = int(float32(gtx.Constraints.Max.X) * 0.85)
		gtx.Constraints.Max.Y = int(float32(gtx.Constraints.Max.Y) * 0.85)
		return modalContent.DrawContent(gtx, widget)
	}
}
