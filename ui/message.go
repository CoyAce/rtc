package ui

import (
	"image"
	"image/color"
	"math"
	"time"

	"gioui.org/font"
	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"gioui.org/x/component"
	"golang.org/x/exp/shiny/materialdesign/icons"
)

type MessageList struct {
	layout.List
	*material.Theme
	Messages     []Message
	ScrollToEnd  bool
	FirstVisible bool
}

type State uint16

const (
	Stateless State = iota
	Sent
	Read
)

type Message struct {
	State
	UUID      string
	Text      string
	Sender    string
	CreatedAt time.Time
}

var avatar Avatar

func (m *Message) Layout(gtx layout.Context, theme *material.Theme) (d layout.Dimensions) {
	if m.Text == "" {
		return d
	}

	margins := layout.Inset{Top: unit.Dp(24), Bottom: unit.Dp(0), Left: unit.Dp(8), Right: unit.Dp(8)}
	return margins.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		flex := layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle, Spacing: layout.SpaceEnd}
		if m.isMe() {
			flex.Spacing = layout.SpaceStart
		}
		return flex.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				if m.isMe() {
					return layout.Dimensions{}
				}
				return avatar.Layout(gtx)
			}),
			layout.Rigid(layout.Spacer{Width: unit.Dp(2)}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return m.drawMessage(gtx, theme)
			}),
			layout.Rigid(layout.Spacer{Width: unit.Dp(2)}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				if m.isMe() {
					return avatar.Layout(gtx)
				}
				return layout.Dimensions{}
			}))
	})
}

func (m *Message) isMe() bool {
	return m.UUID == m.Sender
}

func (m *Message) AddTo(list *MessageList) {
	list.Messages = append(list.Messages, *m)
}

func (m *Message) drawMessage(gtx layout.Context, theme *material.Theme) layout.Dimensions {
	flex := layout.Flex{Axis: layout.Vertical, Alignment: layout.Start}
	if m.isMe() {
		flex.Alignment = layout.End
	}
	return flex.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return m.drawName(gtx, theme)
		}),
		// state and message
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			flex := layout.Flex{Spacing: layout.SpaceEnd, Alignment: layout.Middle}
			if m.isMe() {
				flex.Spacing = layout.SpaceStart
			}
			return flex.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return m.drawState(gtx, theme)
				}),
				layout.Rigid(layout.Spacer{Width: unit.Dp(2)}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return m.drawContent(gtx, theme)
				}),
			)
		}),
	)
}

func (m *Message) drawContent(gtx layout.Context, theme *material.Theme) layout.Dimensions {
	if m.Text != "" {
		// calculate text size for later use
		macro := op.Record(gtx.Ops)
		d := layout.UniformInset(unit.Dp(12)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min.X = 0
			gtx.Constraints.Max.X = int(float32(gtx.Constraints.Max.X) / 1.5)
			bd := material.Body1(theme, m.Text)
			return bd.Layout(gtx)
		})
		call := macro.Stop()
		// draw border
		bgColor := theme.ContrastBg
		radius := gtx.Dp(16)
		sE, sW, nW, nE := radius, radius, radius, radius
		if m.isMe() {
			nE = 0
			bgColor.A = 128
		} else {
			nW = 0
			bgColor.A = 50
		}
		defer clip.RRect{Rect: image.Rectangle{
			Max: d.Size,
		}, SE: sE, SW: sW, NW: nW, NE: nE}.Push(gtx.Ops).Pop()
		component.Rect{Color: bgColor, Size: d.Size}.Layout(gtx)
		// draw text
		call.Add(gtx.Ops)
		return d
	}
	return layout.Dimensions{}
}

func (m *Message) drawState(gtx layout.Context, theme *material.Theme) layout.Dimensions {
	if m.isMe() {
		iconColor := theme.ContrastBg
		var icon *widget.Icon
		switch m.State {
		case Stateless:
			icon, _ = widget.NewIcon(icons.AlertErrorOutline)
			iconColor = color.NRGBA{R: 255, G: 48, B: 48, A: 255}
		case Sent:
			icon, _ = widget.NewIcon(icons.ActionDone)
		case Read:
			icon, _ = widget.NewIcon(icons.ActionDoneAll)
		}
		return icon.Layout(gtx, iconColor)
	}
	return layout.Dimensions{}
}

func (m *Message) drawName(gtx layout.Context, theme *material.Theme) layout.Dimensions {
	timeVal := m.CreatedAt
	timeMsg := timeVal.Local().Format("Mon, Jan 2, 3:04 PM")
	var msg string
	if m.isMe() {
		msg = timeMsg + " " + m.Sender
	} else {
		msg = m.Sender + " " + timeMsg
	}
	label := material.Label(theme, theme.TextSize*0.70, msg)
	label.MaxLines = 1
	label.Color = theme.ContrastBg
	label.Color.A = uint8(int(math.Abs(float64(label.Color.A)-50)) % 256)
	label.Font.Weight = font.Bold
	label.Font.Style = font.Italic
	margins := layout.Inset{Bottom: unit.Dp(8.0)}
	return margins.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		flex := layout.Flex{Spacing: layout.SpaceEnd}
		if m.isMe() {
			flex.Spacing = layout.SpaceStart
		}
		return flex.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return label.Layout(gtx)
			}))
	})
}

func (l *MessageList) Layout(gtx layout.Context) layout.Dimensions {
	// Process events using the key, &messageList
	for {
		_, ok := gtx.Event(
			pointer.Filter{
				Target: l,
				Kinds:  pointer.Press,
			},
		)
		if !ok {
			break
		}
		// get focus from editor
		gtx.Execute(key.FocusCmd{})
	}
	for {
		_, ok := gtx.Event(
			pointer.Filter{
				Target:  l,
				Kinds:   pointer.Scroll,
				ScrollY: pointer.ScrollRange{Min: -1, Max: +1},
			},
		)
		if !ok {
			break
		}
		l.ScrollToEnd = false
	}
	// We visualize the text using a list where each paragraph is a separate item.
	l.List.ScrollToEnd = l.FirstVisible || l.ScrollToEnd
	if l.ScrollToEnd {
		l.List.Position = layout.Position{BeforeEnd: false}
	}
	dimensions := l.List.Layout(gtx, len(l.Messages), func(gtx layout.Context, index int) layout.Dimensions {
		return l.Messages[index].Layout(gtx, l.Theme)
	})
	// at end of list
	if !l.List.Position.BeforeEnd {
		// if at end and first item visible, scroll to end
		l.FirstVisible = l.List.Position.First == 0
	}
	// Confine the area of interest
	defer clip.Rect(image.Rectangle{Max: dimensions.Size}).Push(gtx.Ops).Pop()
	// Use pointer.PassOp to allow pointer events to pass through an input area to those underneath it.
	defer pointer.PassOp{}.Push(gtx.Ops).Pop()
	// Declare `tag` as being one of the targets.
	event.Op(gtx.Ops, l)
	return dimensions
}
