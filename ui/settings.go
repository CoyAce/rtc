package ui

import (
	"image"
	"log"
	"os"
	"rtc/assets/fonts"
	"rtc/core"
	ui "rtc/ui/layout"
	"time"

	"gioui.org/font"
	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"gioui.org/x/component"
)

type SettingsForm struct {
	*material.Theme
	avatar           Avatar
	modalContent     *ui.ModalContent
	onSuccess        func(gtx layout.Context)
	nicknameEditor   *component.TextField
	signEditor       *component.TextField
	serverAddrEditor *component.TextField
	submitButton     IconButton
}

func NewSettingsForm(onSuccess func(gtx layout.Context)) *SettingsForm {
	s := &SettingsForm{
		Theme:            fonts.NewTheme(),
		avatar:           Avatar{UUID: client.FullID(), Size: 64, Editable: true, Theme: theme, OnChange: SyncSelectedIcon},
		onSuccess:        onSuccess,
		nicknameEditor:   &component.TextField{Editor: widget.Editor{}},
		signEditor:       &component.TextField{Editor: widget.Editor{}},
		serverAddrEditor: &component.TextField{Editor: widget.Editor{}},
		submitButton:     IconButton{Theme: theme, Icon: actionDoneIcon, Enabled: true},
	}
	s.Theme.TextSize = 0.75 * s.Theme.TextSize
	s.submitButton.OnClick = func(gtx layout.Context) {
		oldUUID := client.FullID()
		nicknameChanged := s.nicknameEditor.Text() != client.Nickname
		if nicknameChanged {
			client.SetNickName(s.nicknameEditor.Text())
			newUUID := client.FullID()
			renameOldPathToNewPath(oldUUID, newUUID)
			// update cache
			copyOldCacheEntryToNewCache(oldUUID, newUUID)
		}
		client.SetSign(s.signEditor.Text())
		client.SetServerAddr(s.serverAddrEditor.Text())
		// SendSign first, bind uuid to sign
		client.SendSign()
		if nicknameChanged {
			// then sync icon
			SyncSelectedIcon(s.avatar.Image)
		}
		client.Store()
		s.onSuccess(gtx)
	}
	s.modalContent = ui.NewModalContent(theme, func() {
		modal.Dismiss(nil)
		s.nicknameEditor.Clear()
		s.signEditor.Clear()
		s.serverAddrEditor.Clear()
	})
	return s
}

func copyOldCacheEntryToNewCache(oldUUID string, newUUID string) {
	avatar := avatarCache[oldUUID]
	if avatar != nil {
		avatarCache[newUUID] = avatar
	}
}

func renameOldPathToNewPath(oldUUID string, newUUID string) {
	oldPath := core.GetDir(oldUUID)
	newPath := core.GetDir(newUUID)
	err := os.Rename(oldPath, newPath)
	if err != nil {
		log.Printf("Failed to rename: %v", err)
	}
}

func (s *SettingsForm) Layout(gtx layout.Context) layout.Dimensions {
	s.processClick(gtx)
	if len(s.nicknameEditor.Text()) == 0 && !gtx.Focused(&s.nicknameEditor.Editor) {
		s.nicknameEditor.SetText(client.Nickname)
	}
	if len(s.signEditor.Text()) == 0 && !gtx.Focused(&s.signEditor.Editor) {
		s.signEditor.SetText(client.Sign)
	}
	if len(s.serverAddrEditor.Text()) == 0 && !gtx.Focused(&s.serverAddrEditor.Editor) {
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
		return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Baseline}.Layout(gtx,
			layout.Flexed(0.4, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Spacing: layout.SpaceStart}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						gtx.Constraints.Max.X = int(float32(gtx.Constraints.Max.X) * 0.8)
						labelWidget := material.Label(s.Theme, s.TextSize, label)
						labelWidget.Font.Weight = font.Bold
						return labelWidget.Layout(gtx)
					}),
				)
			}),
			layout.Rigid(layout.Spacer{Width: unit.Dp(8)}.Layout),
			layout.Flexed(0.6, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Spacing: layout.SpaceEnd}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						gtx.Constraints.Max.X = int(float32(gtx.Constraints.Max.X) * 0.8)
						return widget(gtx)
					}),
				)
			}))
	}
}

func (s *SettingsForm) ShowWithModal(gtx layout.Context) {
	iconStackAnimation.Disappear(gtx.Now)
	modal.Show(s.ZoomInWithModalContent, nil, component.VisibilityAnimation{
		Duration: time.Millisecond * 250,
		State:    component.Invisible,
		Started:  time.Time{},
	})
}

func (s *SettingsForm) ZoomInWithModalContent(gtx layout.Context) layout.Dimensions {
	gtx.Constraints.Max.X = int(float32(gtx.Constraints.Max.X) * 0.85)
	gtx.Constraints.Max.Y = int(float32(gtx.Constraints.Max.Y) * 0.85)
	return s.modalContent.DrawContent(gtx, s.Layout)
}
