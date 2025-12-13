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
	"golang.org/x/exp/shiny/materialdesign/colornames"
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
	*material.Theme
	UUID      string
	Text      string
	Sender    string
	CreatedAt time.Time
}

type MessageEditor struct {
	*material.Theme
	InputField     *component.TextField
	submitButton   widget.Clickable
	expandButton   widget.Clickable
	collapseButton widget.Clickable
}

type IconStack struct {
	*material.Theme
	Icons   []*widget.Icon
	button  widget.Clickable
	buttons map[*widget.Icon]*widget.Clickable
}

var submitIcon, _ = widget.NewIcon(icons.ContentSend)
var expandIcon, _ = widget.NewIcon(icons.NavigationUnfoldMore)
var collapseIcon, _ = widget.NewIcon(icons.NavigationUnfoldLess)
var voiceMessageIcon, _ = widget.NewIcon(icons.AVMic)
var audioCallIcon, _ = widget.NewIcon(icons.CommunicationPhone)
var videoCallIcon, _ = widget.NewIcon(icons.AVVideoCall)

var animation = component.VisibilityAnimation{
	Duration: time.Millisecond * 250,
	State:    component.Invisible,
	Started:  time.Time{},
}

var avatar Avatar

func NewIconStack(theme *material.Theme) *IconStack {
	return &IconStack{Theme: theme, Icons: []*widget.Icon{videoCallIcon, audioCallIcon, voiceMessageIcon}}
}

func (m *Message) Layout(gtx layout.Context) (d layout.Dimensions) {
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
					return d
				}
				return avatar.Layout(gtx)
			}),
			layout.Rigid(layout.Spacer{Width: unit.Dp(2)}.Layout),
			layout.Rigid(m.drawMessage),
			layout.Rigid(layout.Spacer{Width: unit.Dp(2)}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				if m.isMe() {
					return avatar.Layout(gtx)
				}
				return d
			}))
	})
}

func (m *Message) isMe() bool {
	return m.UUID == m.Sender
}

func (m *Message) AddTo(list *MessageList) {
	list.Messages = append(list.Messages, *m)
}

func (m *Message) drawMessage(gtx layout.Context) layout.Dimensions {
	flex := layout.Flex{Axis: layout.Vertical, Alignment: layout.Start}
	if m.isMe() {
		flex.Alignment = layout.End
	}
	return flex.Layout(gtx,
		layout.Rigid(m.drawName),
		// state and message
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			flex := layout.Flex{Spacing: layout.SpaceEnd, Alignment: layout.Middle}
			if m.isMe() {
				flex.Spacing = layout.SpaceStart
			}
			return flex.Layout(gtx,
				layout.Rigid(m.drawState),
				layout.Rigid(layout.Spacer{Width: unit.Dp(2)}.Layout),
				layout.Rigid(m.drawContent),
			)
		}),
	)
}

func (m *Message) drawContent(gtx layout.Context) layout.Dimensions {
	if m.Text != "" {
		// calculate text size for later use
		macro := op.Record(gtx.Ops)
		d := layout.UniformInset(unit.Dp(12)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min.X = 0
			gtx.Constraints.Max.X = int(float32(gtx.Constraints.Max.X) / 1.5)
			bd := material.Body1(m.Theme, m.Text)
			return bd.Layout(gtx)
		})
		call := macro.Stop()
		// draw border
		bgColor := m.Theme.ContrastBg
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

func (m *Message) drawState(gtx layout.Context) layout.Dimensions {
	if m.isMe() {
		iconColor := m.Theme.ContrastBg
		var icon *widget.Icon
		switch m.State {
		case Stateless:
			icon, _ = widget.NewIcon(icons.AlertErrorOutline)
			iconColor = color.NRGBA(colornames.Red500)
		case Sent:
			icon, _ = widget.NewIcon(icons.ActionDone)
		case Read:
			icon, _ = widget.NewIcon(icons.ActionDoneAll)
		}
		return icon.Layout(gtx, iconColor)
	}
	return layout.Dimensions{}
}

func (m *Message) drawName(gtx layout.Context) layout.Dimensions {
	timeVal := m.CreatedAt
	timeMsg := timeVal.Local().Format("Mon, Jan 2, 3:04 PM")
	var msg string
	if m.isMe() {
		msg = timeMsg + " " + m.Sender
	} else {
		msg = m.Sender + " " + timeMsg
	}
	label := material.Label(m.Theme, m.Theme.TextSize*0.70, msg)
	label.MaxLines = 1
	label.Color = m.Theme.ContrastBg
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
	l.processClick(gtx)
	l.processScroll(gtx)
	// We visualize the text using a list where each paragraph is a separate item.
	l.List.ScrollToEnd = l.FirstVisible || l.ScrollToEnd
	if l.ScrollToEnd {
		l.List.Position = layout.Position{BeforeEnd: false}
	}
	dimensions := l.List.Layout(gtx, len(l.Messages), func(gtx layout.Context, index int) layout.Dimensions {
		return l.Messages[index].Layout(gtx)
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

func (l *MessageList) processScroll(gtx layout.Context) {
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
}

func (l *MessageList) processClick(gtx layout.Context) {
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
}

func (e *MessageEditor) Layout(gtx layout.Context) layout.Dimensions {
	// Render with flexbox layout:
	return layout.Flex{
		// Vertical alignment, from top to bottom
		Axis: layout.Vertical,
		// Empty space is left at the start, i.e. at the top
		Spacing: layout.SpaceStart,
	}.Layout(gtx,
		// Rigid to hold message input field and submit button
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			// Define margins around the flex item using layout.Inset
			margins := layout.Inset{Top: unit.Dp(8.0), Left: unit.Dp(8.0), Right: unit.Dp(8), Bottom: unit.Dp(15)}
			return margins.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{
					Axis:      layout.Horizontal,
					Spacing:   layout.SpaceBetween,
					Alignment: layout.End,
				}.Layout(gtx,
					// text input
					layout.Flexed(1.0, func(gtx layout.Context) layout.Dimensions {
						return e.InputField.Layout(gtx, e.Theme, "Message")
					}),
					// submit button
					layout.Rigid(e.drawSubmitButton),
					// expand button
					layout.Rigid(e.drawExtraButton),
				)
			})
		}),
	)
}

func (e *MessageEditor) drawExtraButton(gtx layout.Context) layout.Dimensions {
	margins := layout.Inset{Left: unit.Dp(8.0)}
	return margins.Layout(
		gtx,
		func(gtx layout.Context) layout.Dimensions {
			btn := &e.expandButton
			icon := expandIcon
			if e.collapseButton.Clicked(gtx) {
				animation.Disappear(gtx.Now)
			}
			if e.expandButton.Clicked(gtx) {
				animation.Appear(gtx.Now)
			}
			if animation.Revealed(gtx) != 0 {
				btn = &e.collapseButton
				icon = collapseIcon
			}
			return material.IconButtonStyle{
				Background: e.Theme.ContrastBg,
				Color:      e.Theme.ContrastFg,
				Icon:       icon,
				Size:       unit.Dp(24.0),
				Button:     btn,
				Inset:      layout.UniformInset(unit.Dp(9)),
			}.Layout(gtx)
		},
	)
}

func (e *MessageEditor) drawSubmitButton(gtx layout.Context) layout.Dimensions {
	margins := layout.Inset{Left: unit.Dp(8.0)}
	return margins.Layout(gtx,
		func(gtx layout.Context) layout.Dimensions {
			return material.IconButtonStyle{
				Background: e.Theme.ContrastBg,
				Color:      e.Theme.ContrastFg,
				Icon:       submitIcon,
				Size:       unit.Dp(24.0),
				Button:     &e.submitButton,
				Inset:      layout.UniformInset(unit.Dp(9)),
			}.Layout(gtx)
		},
	)
}

func (e *MessageEditor) Submitted(gtx layout.Context) bool {
	return e.submitButton.Clicked(gtx) || e.submittedByCarriageReturn(gtx)
}

func (e *MessageEditor) submittedByCarriageReturn(gtx layout.Context) (submit bool) {
	for {
		ev, ok := e.InputField.Editor.Update(gtx)
		if _, submit = ev.(widget.SubmitEvent); submit {
			break
		}
		if !ok {
			break
		}
	}
	return submit
}

func (s *IconStack) Layout(gtx layout.Context) layout.Dimensions {
	layout.Stack{Alignment: layout.SE}.Layout(gtx,
		layout.Stacked(func(gtx layout.Context) layout.Dimensions {
			offset := image.Pt(-gtx.Dp(8), -gtx.Dp(64))
			op.Offset(offset).Add(gtx.Ops)
			progress := animation.Revealed(gtx)
			macro := op.Record(gtx.Ops)
			d := s.button.Layout(gtx, s.drawIconsStackItems)
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

func (s *IconStack) drawIconsStackItems(gtx layout.Context) layout.Dimensions {
	if s.buttons == nil {
		s.buttons = make(map[*widget.Icon]*widget.Clickable)
	}
	flex := layout.Flex{Axis: layout.Vertical}
	var children []layout.FlexChild
	for _, icon := range s.Icons {
		if btn := s.buttons[icon]; btn == nil {
			s.buttons[icon] = &widget.Clickable{}
		}
		children = append(children, layout.Rigid(s.drawIconButton(s.buttons[icon], icon)))
		children = append(children, layout.Rigid(layout.Spacer{Height: unit.Dp(8)}.Layout))
	}
	children = children[:len(children)-1]
	return flex.Layout(gtx, children...)
}

func (s *IconStack) drawIconButton(btn *widget.Clickable, icon *widget.Icon) func(gtx layout.Context) layout.Dimensions {
	return func(gtx layout.Context) layout.Dimensions {
		inset := layout.Inset{Left: unit.Dp(8.0)}
		return inset.Layout(
			gtx,
			func(gtx layout.Context) layout.Dimensions {
				return material.IconButtonStyle{
					Background: s.Theme.ContrastBg,
					Color:      s.Theme.ContrastFg,
					Icon:       icon,
					Size:       unit.Dp(24.0),
					Button:     btn,
					Inset:      layout.UniformInset(unit.Dp(9)),
				}.Layout(gtx)
			},
		)
	}
}
