package view

import (
	"image"
	"io"
	"log"
	"mushin/assets/fonts"
	"mushin/assets/icons"
	"os"
	"path/filepath"
	"strings"
	"time"

	modal "mushin/ui/layout"

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
		avatar:           Avatar{UUID: wi.DefaultClient.ID(), Size: 64, Editable: true, Theme: fonts.DefaultTheme, OnChange: SyncIcon},
		onSuccess:        onSuccess,
		nicknameEditor:   &component.TextField{Editor: widget.Editor{}},
		signEditor:       &component.TextField{Editor: widget.Editor{}},
		serverAddrEditor: &component.TextField{Editor: widget.Editor{}},
		submitButton:     IconButton{Theme: fonts.DefaultTheme, Icon: icons.ActionDoneIcon, Enabled: true},
	}
	s.Theme.TextSize = 0.75 * s.Theme.TextSize
	s.submitButton.OnClick = func() {
		wi.DefaultClient.SetSign(s.signEditor.Text())
		wi.DefaultClient.SetServerAddr(s.serverAddrEditor.Text())
		nicknameChanged := s.nicknameEditor.Text() != wi.DefaultClient.Nickname
		oldUUID := wi.DefaultClient.ID()
		if nicknameChanged {
			// Send invisible message to record nickname change.
			MessageBox <- NewInvisibleMessage()
			wi.DefaultClient.MultiTrack(&wi.SignBody{Sign: wi.DefaultClient.Sign, UUID: oldUUID}, wi.FullRange)
			wi.DefaultClient.SetNickName(s.nicknameEditor.Text())
			newUUID := wi.DefaultClient.ID()
			renamePath(oldUUID, newUUID)
			go func() {
				copyThenReloadIcon(newUUID, oldUUID)
				CopyOpusFiles(oldUUID, newUUID)
			}()
			// update cache
			copyCacheEntry(oldUUID, newUUID)
		}
		go func() {
			// confirmed sign in
			wi.DefaultClient.SignIn()
			wi.DefaultClient.Pull()
			if nicknameChanged {
				wi.DefaultClient.SyncName(oldUUID)
			}
		}()
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

func copyCacheEntry(oldUUID string, newUUID string) {
	avatar := AvatarCache.LoadOrElseNew(oldUUID)
	AvatarCache.Add(newUUID, avatar)
}

func renamePath(oldUUID string, newUUID string) {
	oldPath := GetDir(oldUUID)
	newPath := GetDir(newUUID)
	err := os.RemoveAll(newPath)
	if err != nil {
		log.Printf("Failed to rename: %v", err)
	}
	err = os.Rename(oldPath, newPath)
	if err != nil {
		log.Printf("Failed to rename: %v", err)
	}
}

// CopyOpusFiles copy opus files from newUUID to oldUUID
func CopyOpusFiles(oldUUID string, newUUID string) {
	src := GetDir(newUUID)
	dst := GetDir(oldUUID)
	if err := os.MkdirAll(dst, 0755); err != nil {
		log.Printf("create destination dir failed: %v", err)
	}

	err := filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		if strings.HasSuffix(strings.ToLower(path), ".opus") {
			fileName := filepath.Base(path)
			dstPath := filepath.Join(dst, fileName)

			if err = copyFile(path, dstPath); err != nil {
				log.Printf("copy %s failed: %v", path, err)
			}
		}
		return nil
	})
	if err != nil {
		log.Printf("walk %s failed: %v", src, err)
	}
}

func copyFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	dstFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	_, err = io.Copy(dstFile, srcFile)
	return err
}

// copyThenReloadIcon copy icon from oldUUID to newUUID
func copyThenReloadIcon(oldUUID string, newUUID string) {
	paths := []string{
		GetPath(oldUUID, "icon.png"), GetPath(oldUUID, "icon.gif"),
	}
	for _, path := range paths {
		_, err := os.Stat(path)
		if err != nil {
			continue
		}
		target := GetPath(newUUID, filepath.Base(path))
		wi.Mkdir(filepath.Dir(target))
		err = copyFile(path, target)
		if err != nil {
			log.Printf("copy %s failed: %v", path, err)
		}
		avatar := AvatarCache.LoadOrElseNew(newUUID)
		if filepath.Ext(path) == ".gif" {
			avatar.Reload(GIF_IMG)
		} else {
			avatar.Reload(IMG)
		}
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
