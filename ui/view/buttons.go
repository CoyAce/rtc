package view

import (
	"image"
	"image/color"
	"log"
	"path/filepath"
	"rtc/assets/fonts"
	"rtc/core"
	"runtime"
	"strings"
	"time"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"gioui.org/x/component"
	"golang.org/x/exp/shiny/materialdesign/colornames"
)

type IconButton struct {
	*material.Theme
	Icon    *widget.Icon
	Enabled bool
	OnClick func(gtx layout.Context)
	button  widget.Clickable
}

func (b *IconButton) Layout(gtx layout.Context) layout.Dimensions {
	if b.button.Clicked(gtx) && b.OnClick != nil {
		b.OnClick(gtx)
	}
	bg := b.Theme.ContrastBg
	if !b.Enabled {
		bg = color.NRGBA(colornames.Grey500)
	}
	return material.IconButtonStyle{
		Background: bg,
		Color:      b.Theme.ContrastFg,
		Icon:       b.Icon,
		Size:       unit.Dp(24.0),
		Button:     &b.button,
		Inset:      layout.UniformInset(unit.Dp(9)),
	}.Layout(gtx)
}

type IconStack struct {
	*material.Theme
	IconButtons []*IconButton
	button      widget.Clickable
}

func (s *IconStack) drawIconStackItems(gtx layout.Context) layout.Dimensions {
	flex := layout.Flex{Axis: layout.Vertical}
	var children []layout.FlexChild
	for _, button := range s.IconButtons {
		children = append(children, layout.Rigid(button.Layout))
		children = append(children, layout.Rigid(layout.Spacer{Height: unit.Dp(8)}.Layout))
	}
	return flex.Layout(gtx, children...)
}

func (s *IconStack) Layout(gtx layout.Context) layout.Dimensions {
	layout.Stack{Alignment: layout.SE}.Layout(gtx,
		layout.Stacked(func(gtx layout.Context) layout.Dimensions {
			offset := image.Pt(-gtx.Dp(8), -gtx.Dp(57))
			op.Offset(offset).Add(gtx.Ops)
			progress := iconStackAnimation.Revealed(gtx)
			macro := op.Record(gtx.Ops)
			d := s.button.Layout(gtx, s.drawIconStackItems)
			call := macro.Stop()
			d.Size.Y = int(float32(d.Size.Y) * progress)
			component.Rect{Size: d.Size, Color: color.NRGBA{}}.Layout(gtx)
			defer clip.Rect{Max: d.Size}.Push(gtx.Ops).Pop()
			call.Add(gtx.Ops)
			return d
		}),
	)
	return layout.Dimensions{}
}

var iconStackAnimation = component.VisibilityAnimation{
	Duration: time.Millisecond * 250,
	State:    component.Invisible,
	Started:  time.Time{},
}

func NewIconStack() *IconStack {
	settings := NewSettingsForm(OnSettingsSubmit)
	return &IconStack{Theme: fonts.DefaultTheme,
		IconButtons: []*IconButton{
			{Theme: fonts.DefaultTheme, Icon: settingsIcon, Enabled: true, OnClick: settings.ShowWithModal},
			{Theme: fonts.DefaultTheme, Icon: videoCallIcon},
			{Theme: fonts.DefaultTheme, Icon: audioCallIcon},
			{Theme: fonts.DefaultTheme, Icon: voiceMessageIcon},
			{Theme: fonts.DefaultTheme, Icon: photoLibraryIcon, Enabled: true, OnClick: func(gtx layout.Context) {
				iconStackAnimation.Disappear(gtx.Now)
				go func() {
					img, absolutePath, err := ChooseImageAndDecode()
					if err != nil {
						log.Printf("choose image failed, %v", err)
						return
					}
					filename := filepath.Base(absolutePath)
					// android get displayName, need copy to user space
					if runtime.GOOS == "android" {
						if filepath.Ext(filename) == ".webp" {
							filename = strings.TrimSuffix(filepath.Base(filename), ".webp") + ".png"
						}
						absolutePath = core.GetFilePath(core.DefaultClient.FullID(), filename)
						go core.Save(img, filename)
					}
					imageCache[absolutePath] = &img
					message := &Message{State: Stateless, Theme: fonts.DefaultTheme,
						UUID: core.DefaultClient.FullID(), Type: Image, Filename: absolutePath,
						Sender: core.DefaultClient.FullID(), CreatedAt: time.Now()}
					if filepath.Ext(filename) == ".gif" {
						message.Type = GIF
					}
					MessageBox <- message
					err = core.DefaultClient.SendImage(img, filename)
					if err != nil {
						log.Printf("send image failed, %v", err)
						message.State = Failed
					} else {
						message.State = Sent
					}
				}()
			}},
		},
	}
}
