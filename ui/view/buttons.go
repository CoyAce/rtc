package view

import (
	"image"
	"image/color"
	"rtc/assets/fonts"
	"rtc/internal/audio"
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
	Hidden  bool
	Active  bool
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
	} else if b.Active {
		bg = color.NRGBA(colornames.Red400)
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
	*component.VisibilityAnimation
	IconButtons []*IconButton
}

func (s *IconStack) drawIconStackItems(gtx layout.Context) layout.Dimensions {
	flex := layout.Flex{Axis: layout.Vertical}
	var children []layout.FlexChild
	for _, button := range s.IconButtons {
		if button.Hidden {
			continue
		}
		children = append(children, layout.Rigid(button.Layout))
		children = append(children, layout.Rigid(layout.Spacer{Height: unit.Dp(8)}.Layout))
	}
	return flex.Layout(gtx, children...)
}

func (s *IconStack) Layout(gtx layout.Context) (layout.Dimensions, layout.Dimensions) {
	var d layout.Dimensions
	return layout.Stack{Alignment: layout.SE}.Layout(gtx,
		layout.Stacked(func(gtx layout.Context) layout.Dimensions {
			offset := image.Pt(-gtx.Dp(8), -gtx.Dp(57))
			op.Offset(offset).Add(gtx.Ops)
			progress := s.VisibilityAnimation.Revealed(gtx)
			macro := op.Record(gtx.Ops)
			d = s.drawIconStackItems(gtx)
			call := macro.Stop()
			d.Size.Y = int(float32(d.Size.Y) * progress)
			component.Rect{Size: d.Size, Color: color.NRGBA{}}.Layout(gtx)
			defer clip.Rect{Max: d.Size}.Push(gtx.Ops).Pop()
			call.Add(gtx.Ops)
			return d
		}),
	), d
}

var iconStackAnimation = component.VisibilityAnimation{
	Duration: time.Millisecond * 250,
	State:    component.Invisible,
	Started:  time.Time{},
}

var audioStackAnimation = component.VisibilityAnimation{
	Duration: time.Millisecond * 100,
	State:    component.Invisible,
	Started:  time.Time{},
}

var VoiceMode = false
var AudioCall = false
var audioCall = &IconButton{Theme: fonts.DefaultTheme, Icon: audioCallIcon, Enabled: true}
var audioAcceptCall = &IconButton{Theme: fonts.DefaultTheme, Icon: audioCallIcon, Enabled: true}

func NewIconStack() *IconStack {
	settings := NewSettingsForm(OnSettingsSubmit)
	audioCall.OnClick = MakeAudioCall(audioCall)
	voiceMessage := &IconButton{Theme: fonts.DefaultTheme, Icon: voiceMessageIcon, Enabled: true}
	voiceMessage.OnClick = SwitchBetweenTextAndVoice(voiceMessage)
	return &IconStack{Theme: fonts.DefaultTheme,
		VisibilityAnimation: &iconStackAnimation,
		IconButtons: []*IconButton{
			{Theme: fonts.DefaultTheme, Icon: settingsIcon, Enabled: true, OnClick: settings.ShowWithModal},
			{Theme: fonts.DefaultTheme, Icon: filesIcon},
			{Theme: fonts.DefaultTheme, Icon: photoLibraryIcon, Enabled: true, OnClick: ChooseAndSendPhoto},
			{Theme: fonts.DefaultTheme, Icon: videoCallIcon},
			audioCall,
			voiceMessage,
		},
	}
}

func NewAudioIconStack(streamConfig audio.StreamConfig) *IconStack {
	audioAcceptCall.OnClick = func(gtx layout.Context) {
		audioAcceptCall.Hidden = true
	}
	audioEndCall := &IconButton{Theme: fonts.DefaultTheme, Icon: audioCallIcon, Enabled: true, Active: true}
	audioEndCall.OnClick = func(gtx layout.Context) {
		AudioCall = false
		audioCall.Hidden = false
		audioStackAnimation.Disappear(gtx.Now)
		time.AfterFunc(audioStackAnimation.Duration, func() {
			audioAcceptCall.Hidden = false
		})
	}
	return &IconStack{Theme: fonts.DefaultTheme,
		VisibilityAnimation: &audioStackAnimation,
		IconButtons: []*IconButton{
			audioAcceptCall,
			audioEndCall,
		},
	}
}

func MakeAudioCall(audioCall *IconButton) func(gtx layout.Context) {
	return func(gtx layout.Context) {
		AudioCall = !AudioCall
		if AudioCall {
			audioCall.Hidden = true
			audioAcceptCall.Hidden = true
			time.AfterFunc(iconStackAnimation.Duration, func() {
				audioStackAnimation.Appear(gtx.Now)
			})
		}
	}
}

func SwitchBetweenTextAndVoice(voiceMessage *IconButton) func(gtx layout.Context) {
	return func(gtx layout.Context) {
		iconStackAnimation.Disappear(gtx.Now)
		VoiceMode = !VoiceMode
		if VoiceMode {
			voiceMessage.Icon = chatIcon
		} else {
			voiceMessage.Icon = voiceMessageIcon
		}
	}
}

type ExpandButton struct {
	expandButton   widget.Clickable
	collapseButton widget.Clickable
}

func (e *ExpandButton) Layout(gtx layout.Context) layout.Dimensions {
	margins := layout.Inset{Left: unit.Dp(8.0)}
	return margins.Layout(
		gtx,
		func(gtx layout.Context) layout.Dimensions {
			btn := &e.expandButton
			icon := expandIcon
			if e.collapseButton.Clicked(gtx) {
				iconStackAnimation.Disappear(gtx.Now)
			}
			if e.expandButton.Clicked(gtx) {
				iconStackAnimation.Appear(gtx.Now)
			}
			if iconStackAnimation.Revealed(gtx) != 0 {
				btn = &e.collapseButton
				icon = collapseIcon
			}
			return material.IconButtonStyle{
				Background: fonts.DefaultTheme.ContrastBg,
				Color:      fonts.DefaultTheme.ContrastFg,
				Icon:       icon,
				Size:       unit.Dp(24.0),
				Button:     btn,
				Inset:      layout.UniformInset(unit.Dp(9)),
			}.Layout(gtx)
		},
	)
}
