package view

import (
	"image"
	"log"
	"os"
	"rtc/assets/fonts"
	"rtc/assets/icons"
	"time"

	modal "rtc/ui/layout"

	"gioui.org/font"
	"gioui.org/io/event"
	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"gioui.org/x/component"
	"github.com/CoyAce/wi"
)

type SettingsForm struct {
	*material.Theme
	avatar           Avatar
	modalContent     *modal.ModalContent
	onSuccess        func()
	nicknameEditor   *component.TextField
	signEditor       *component.TextField
	serverAddrEditor *component.TextField
	submitButton     IconButton
	lastItemFocused  bool
}

func NewSettingsForm(onSuccess func()) *SettingsForm {
	s := &SettingsForm{
		Theme:            fonts.NewTheme(),
		avatar:           Avatar{UUID: wi.DefaultClient.FullID(), Size: 64, Editable: true, Theme: fonts.DefaultTheme, OnChange: SyncIcon},
		onSuccess:        onSuccess,
		nicknameEditor:   &component.TextField{Editor: widget.Editor{}},
		signEditor:       &component.TextField{Editor: widget.Editor{}},
		serverAddrEditor: &component.TextField{Editor: widget.Editor{}},
		submitButton:     IconButton{Theme: fonts.DefaultTheme, Icon: icons.ActionDoneIcon, Enabled: true},
	}
	s.Theme.TextSize = 0.75 * s.Theme.TextSize
	s.submitButton.OnClick = func() {
		oldUUID := wi.DefaultClient.FullID()
		nicknameChanged := s.nicknameEditor.Text() != wi.DefaultClient.Nickname
		if nicknameChanged {
			wi.DefaultClient.SetNickName(s.nicknameEditor.Text())
			newUUID := wi.DefaultClient.FullID()
			renameOldPathToNewPath(oldUUID, newUUID)
			// update cache
			copyOldCacheEntryToNewCache(oldUUID, newUUID)
		}
		wi.DefaultClient.SetSign(s.signEditor.Text())
		wi.DefaultClient.SetServerAddr(s.serverAddrEditor.Text())
		// SendSign first, bind uuid to sign
		wi.DefaultClient.SendSign()
		if nicknameChanged && s.avatar.AvatarType != Default {
			// then sync icon
			SyncIcon()
		}
		wi.DefaultClient.Store()
		s.onSuccess()
	}
	s.modalContent = modal.NewModalContent(fonts.DefaultTheme, func() {
		modal.DefaultModal.Dismiss(nil)
		s.nicknameEditor.Clear()
		s.signEditor.Clear()
		s.serverAddrEditor.Clear()
	})
	return s
}

func copyOldCacheEntryToNewCache(oldUUID string, newUUID string) {
	avatar := AvatarCache.LoadOrElseNew(oldUUID)
	AvatarCache.Add(newUUID, avatar)
}

func renameOldPathToNewPath(oldUUID string, newUUID string) {
	oldPath := GetDir(oldUUID)
	newPath := GetDir(newUUID)
	err := os.Rename(oldPath, newPath)
	if err != nil {
		log.Printf("Failed to rename: %v", err)
	}
}

func (s *SettingsForm) Layout(gtx layout.Context) layout.Dimensions {
	if len(s.nicknameEditor.Text()) == 0 && !gtx.Focused(&s.nicknameEditor.Editor) {
		s.nicknameEditor.SetText(wi.DefaultClient.Nickname)
	}
	if len(s.signEditor.Text()) == 0 && !gtx.Focused(&s.signEditor.Editor) {
		s.signEditor.SetText(wi.DefaultClient.Sign)
	}
	lastItemFocused := gtx.Focused(&s.serverAddrEditor.Editor)
	if len(s.serverAddrEditor.Text()) == 0 && !lastItemFocused {
		s.serverAddrEditor.SetText(wi.DefaultClient.ServerAddr)
	}
	focusedEvent := !s.lastItemFocused && lastItemFocused
	s.modalContent.ScrollToEnd = s.modalContent.Position.BeforeEnd && focusedEvent
	if s.modalContent.ScrollToEnd {
		s.lastItemFocused = true
	}
	if !lastItemFocused {
		s.lastItemFocused = false
	}

	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	dimensions := layout.Flex{
		Spacing: layout.SpaceSides,
	}.Layout(gtx,
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
	event.Op(gtx.Ops, s)
	return dimensions
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

func (s *SettingsForm) ShowWithModal() {
	modal.DefaultModal.Show(s.ZoomInWithModalContent, nil, component.VisibilityAnimation{
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
