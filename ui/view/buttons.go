package view

import (
	"image"
	"image/color"
	"rtc/assets/fonts"
	"rtc/assets/icons"
	"time"

	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"gioui.org/x/component"
	"golang.org/x/exp/shiny/materialdesign/colornames"
)

type Mode uint8

const (
	None Mode = iota
	Accept
	Decline
)

type IconButton struct {
	*material.Theme
	Icon    *widget.Icon
	Enabled bool
	Hidden  bool
	Mode
	OnClick func()
	button  widget.Clickable
}

func (b *IconButton) Layout(gtx layout.Context) layout.Dimensions {
	if b.button.Clicked(gtx) && b.OnClick != nil {
		b.OnClick()
	}
	bg := b.Theme.ContrastBg
	if !b.Enabled {
		bg = color.NRGBA(colornames.Grey500)
	}
	switch b.Mode {
	case Accept:
		bg = color.NRGBA(colornames.Green400)
	case Decline:
		bg = color.NRGBA(colornames.Red400)
	default:
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
	Sticky      bool
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
	s.update(gtx)
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
			event.Op(gtx.Ops, s)
			call.Add(gtx.Ops)
			return d
		}),
	), d
}

func (s *IconStack) update(gtx layout.Context) {
	if !s.Sticky && s.State == component.Visible && !gtx.Focused(s) {
		gtx.Execute(key.FocusCmd{Tag: s})
		gtx.Execute(op.InvalidateCmd{})
	}
	for {
		e, ok := gtx.Event(
			key.FocusFilter{Target: s},
		)
		if !ok {
			break
		}
		switch e := e.(type) {
		case key.FocusEvent:
			if !e.Focus && !s.Sticky {
				s.VisibilityAnimation.Disappear(gtx.Now)
			}
		}
	}
}

var iconStackAnimation = component.VisibilityAnimation{
	Duration: time.Millisecond * 250,
	State:    component.Invisible,
	Started:  time.Time{},
}

func NewIconStack(modeSwitch func(*IconButton) func(), appendFile func(mapping *FileDescription)) *IconStack {
	settings := NewSettingsForm(OnSettingsSubmit)
	audioMakeButton.OnClick = MakeAudioCall(audioMakeButton)
	voiceMessageSwitch := &IconButton{Theme: fonts.DefaultTheme, Icon: icons.VoiceMessageIcon, Enabled: true}
	voiceMessageSwitch.OnClick = modeSwitch(voiceMessageSwitch)
	return &IconStack{
		Sticky:              false,
		Theme:               fonts.DefaultTheme,
		VisibilityAnimation: &iconStackAnimation,
		IconButtons: []*IconButton{
			{Theme: fonts.DefaultTheme, Icon: icons.SettingsIcon, Enabled: true, OnClick: settings.ShowWithModal},
			{Theme: fonts.DefaultTheme, Icon: icons.FilesIcon, Enabled: true, OnClick: ChooseAndSendFile(appendFile)},
			{Theme: fonts.DefaultTheme, Icon: icons.PhotoLibraryIcon, Enabled: true, OnClick: ChooseAndSendPhoto},
			{Theme: fonts.DefaultTheme, Icon: icons.VideoCallIcon},
			audioMakeButton,
			voiceMessageSwitch,
		},
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
			icon := icons.ExpandIcon
			if e.collapseButton.Clicked(gtx) {
				iconStackAnimation.Disappear(gtx.Now)
			}
			if e.expandButton.Clicked(gtx) {
				iconStackAnimation.Appear(gtx.Now)
			}
			if iconStackAnimation.Revealed(gtx) != 0 {
				btn = &e.collapseButton
				icon = icons.CollapseIcon
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
