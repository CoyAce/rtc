package view

import (
	"bufio"
	"encoding/json"
	"image"
	"image/color"
	"io"
	"log"
	"math"
	"os"
	"path/filepath"
	"rtc/assets"
	"rtc/assets/fonts"
	"rtc/core"
	ui "rtc/ui/layout"
	text "rtc/ui/layout/component"
	"strings"
	"time"

	"gioui.org/font"
	"gioui.org/gesture"
	"gioui.org/io/clipboard"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"gioui.org/x/component"
	"golang.org/x/exp/shiny/materialdesign/colornames"
)

type MessageList struct {
	ui.List
	*material.Theme
	widget.Clickable
	Messages []*Message
}

type MessageKeeper struct {
	MessageChannel chan *Message
	buffer         []*Message
	ready          chan struct{}
	timer          *time.Timer
}

func (a *MessageKeeper) Loop() {
	a.ready = make(chan struct{}, 1)
	duration := time.Second
	a.timer = time.NewTimer(duration)
	for {
		a.timer.Reset(duration)
		select {
		case msg := <-a.MessageChannel:
			a.buffer = append(a.buffer, msg)
		case <-a.timer.C:
			a.ready <- struct{}{}
		case <-a.ready:
			if len(a.buffer) == 0 {
				continue
			}
			a.Append()
		}
	}
}

func (a *MessageKeeper) Append() {
	filePath := core.GetFilePath(core.DefaultClient.FullID(), "message.log")
	file, err := os.OpenFile(filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Printf("error opening file: %v", err)
	}
	defer file.Close()
	for _, msg := range a.buffer {
		a.writeJson(file, msg)
	}
	a.buffer = a.buffer[:0]
}

func (a *MessageKeeper) writeJson(file *os.File, msg *Message) {
	s, err := json.Marshal(msg)
	if err != nil {
		log.Printf("error marshalling message: %v", err)
		return
	}
	_, err = file.WriteString(string(s) + "\n")
	if err != nil {
		log.Printf("error writing to file: %v", err)
	}
}

func (a *MessageKeeper) Messages() []*Message {
	filePath := core.GetFilePath(core.DefaultClient.FullID(), "message.log")
	f, err := os.Open(filePath)
	if err != nil {
		log.Printf("error opening file: %v", err)
		return []*Message{}
	}
	ret := make([]*Message, 0)
	s := bufio.NewScanner(f)
	for s.Scan() {
		var msg Message
		line := s.Bytes()
		err := json.Unmarshal(line, &msg)
		if err != nil {
			log.Printf("error unmarshalling message: %v", err)
		}
		ed := widget.Editor{ReadOnly: true}
		ed.SetText(msg.Text)
		msg.Editor = &ed
		msg.Theme = fonts.DefaultTheme
		if msg.State == Stateless {
			msg.State = Failed
		}
		ret = append(ret, &msg)
	}
	return ret
}

type State uint16

const (
	Stateless State = iota
	Failed
	Sent
	Read
)

type MessageType uint16

const (
	Text MessageType = iota
	Image
	GIF
)

// LongPressDuration is the default duration of a long press gesture.
// Override this variable to change the detection threshold.
var LongPressDuration time.Duration = 250 * time.Millisecond

// EventType describes a kind of iteraction with rich text.
type EventType uint8

const (
	Hover EventType = iota
	Unhover
	LongPress
	Click
)

// Event describes an interaction with rich text.
type Event struct {
	Type EventType
	// ClickData is only populated if Type == Clicked
	ClickData gesture.ClickEvent
}

// InteractiveSpan holds the persistent state of rich text that can
// be interacted with by the user. It can report clicks, hovers, and
// long-presses on the text.
type InteractiveSpan struct {
	click        gesture.Click
	pressing     bool
	hovering     bool
	longPressing bool
	longPressed  bool
	pressStarted time.Time
}

func (i *InteractiveSpan) Update(gtx layout.Context) (Event, bool) {
	if i == nil {
		return Event{}, false
	}
	for {
		e, ok := i.click.Update(gtx.Source)
		if !ok {
			break
		}
		switch e.Kind {
		case gesture.KindClick:
			i.pressing = false
			if i.longPressing {
				i.longPressing = false
			} else {
				i.longPressed = false
				return Event{Type: Click, ClickData: e}, true
			}
		case gesture.KindPress:
			i.pressStarted = gtx.Now
			i.pressing = true
		case gesture.KindCancel:
			i.pressing = false
			i.longPressing = false
			i.longPressed = false
		}
	}
	if isHovered := i.click.Hovered(); isHovered != i.hovering {
		i.hovering = isHovered
		if isHovered {
			return Event{Type: Hover}, true
		} else {
			return Event{Type: Unhover}, true
		}
	}

	if !i.longPressing && i.pressing && gtx.Now.Sub(i.pressStarted) > LongPressDuration {
		i.longPressing = true
		i.longPressed = true
		return Event{Type: LongPress}, true
	}
	return Event{}, false
}

// Layout adds the pointer input op for this interactive span and updates its
// state. It uses the most recent pointer.AreaOp as its input area.
func (i *InteractiveSpan) Layout(gtx layout.Context) layout.Dimensions {
	for {
		_, ok := i.Update(gtx)
		if !ok {
			break
		}
	}
	if i.pressing && !i.longPressing {
		gtx.Execute(op.InvalidateCmd{})
	}
	defer clip.Rect{Max: gtx.Constraints.Max}.Push(gtx.Ops).Pop()

	pointer.CursorPointer.Add(gtx.Ops)
	i.click.Add(gtx.Ops)
	return layout.Dimensions{}
}

type Message struct {
	State
	Editor          *widget.Editor
	*material.Theme `json:"-"`
	InteractiveSpan `json:"-"`
	contentCopy     widget.Clickable
	fileDownload    widget.Clickable
	Filename        string
	Type            MessageType
	UUID            string
	Text            string
	Sender          string
	CreatedAt       time.Time
	imageBroken     bool
}

type MessageEditor struct {
	*material.Theme
	InputField     *text.TextField
	submitButton   widget.Clickable
	expandButton   widget.Clickable
	collapseButton widget.Clickable
}

func (m *Message) Layout(gtx layout.Context) (d layout.Dimensions) {
	if m.Type == Text && m.Text == "" {
		return d
	}
	if m.Type == Image && m.Filename == "" {
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
				return m.drawAvatar(gtx, m.Sender)
			}),
			layout.Rigid(layout.Spacer{Width: unit.Dp(2)}.Layout),
			layout.Rigid(m.drawMessage),
			layout.Rigid(layout.Spacer{Width: unit.Dp(2)}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				if m.isMe() {
					return m.drawAvatar(gtx, m.UUID)
				}
				return d
			}))
	})
}

func (m *Message) drawAvatar(gtx layout.Context, uuid string) layout.Dimensions {
	avatar := AvatarCache[uuid]
	if avatar == nil {
		avatar = &Avatar{UUID: uuid}
		AvatarCache[uuid] = avatar
	}
	return avatar.Layout(gtx)
}

func (m *Message) isMe() bool {
	return m.UUID == m.Sender
}

func (m *Message) AddTo(list *MessageList) {
	list.Messages = append(list.Messages, m)
}

func (m *Message) SendTo(messageAppender *MessageKeeper) {
	messageAppender.MessageChannel <- m
}

func (m *Message) drawMessage(gtx layout.Context) layout.Dimensions {
	m.processTextCopy(gtx)
	m.processFileSave(gtx)
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
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					if m.isMe() && m.interactable() {
						return m.drawOperation(gtx)
					}
					return m.drawState(gtx)
				}),
				layout.Rigid(layout.Spacer{Width: unit.Dp(4)}.Layout),
				layout.Rigid(m.drawContent),
				layout.Rigid(layout.Spacer{Width: unit.Dp(4)}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					if m.isMe() || !m.interactable() {
						return layout.Dimensions{}
					}
					return m.drawOperation(gtx)
				}),
			)
		}),
	)
}

func (m *Message) interactable() bool {
	return m.longPressed || m.Editor.SelectionLen() > 0
}

func (m *Message) processTextCopy(gtx layout.Context) {
	if m.contentCopy.Clicked(gtx) {
		textForCopy := m.Text
		if m.Editor.SelectionLen() > 0 {
			textForCopy = m.Editor.SelectedText()
		}
		gtx.Execute(clipboard.WriteCmd{Type: "application/text", Data: io.NopCloser(strings.NewReader(textForCopy))})
	}
	if !gtx.Focused(m.Editor) {
		m.Editor.ClearSelection()
	}
}

func (m *Message) processFileSave(gtx layout.Context) {
	if !m.fileDownload.Clicked(gtx) {
		return
	}
	go func() {
		if m.Filename == "" {
			return
		}
		file, err := DefaultPicker.CreateFile(filepath.Base(m.Filename))
		if err != nil {
			log.Printf("Error creating file for %s: %s", m.Filename, err)
			return
		}
		defer file.Close()
		switch m.Type {
		case Image:
			img, err := m.loadImage()
			if err != nil || img == nil || img == &assets.AppIconImage {
				return
			}
			err = core.EncodeImg(file, m.Filename, *img)
			if err != nil {
				log.Printf("encode file failed, %v", err)
			} else {
				log.Printf("%s saved", m.Filename)
			}
		case GIF:
			gifImg, err := m.loadGif()
			if err != nil || gifImg == nil || gifImg == &EmptyGif {
				return
			}
			core.EncodeGif(file, m.Filename, gifImg.GIF)
		default:
			return
		}
	}()
}

func (m *Message) drawOperation(gtx layout.Context) layout.Dimensions {
	if m.imageBroken {
		return layout.Dimensions{}
	}
	switch m.Type {
	case Text:
		return m.contentCopy.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return contentCopyIcon.Layout(gtx, m.ContrastBg)
		})
	case Image:
		fallthrough
	case GIF:
		return m.fileDownload.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return downloadIcon.Layout(gtx, m.ContrastBg)
		})
	}
	return layout.Dimensions{}
}

func (m *Message) drawContent(gtx layout.Context) layout.Dimensions {
	switch m.Type {
	case Text:
		if m.Text == "" {
			return layout.Dimensions{}
		}
		// calculate text size for later use
		macro := op.Record(gtx.Ops)
		d := layout.UniformInset(unit.Dp(12)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min.X = 0
			gtx.Constraints.Max.X = int(float32(gtx.Constraints.Max.X) / 1.5)
			return material.Editor(m.Theme, m.Editor, "hint").Layout(gtx)
		})
		call := macro.Stop()
		m.drawBorder(gtx, d, call)
		return d
	case Image:
		if m.Filename == "" {
			return layout.Dimensions{}
		}
		img, err := m.loadImage()
		if err != nil || img == nil || img == &assets.AppIconImage {
			return m.drawBrokenImage(gtx)
		}
		return m.drawImage(gtx, *img)
	case GIF:
		if m.Filename == "" {
			return layout.Dimensions{}
		}
		gifImg, err := m.loadGif()
		if err != nil || gifImg == nil || gifImg == &EmptyGif {
			return m.drawBrokenImage(gtx)
		}
		return m.drawGif(gtx, gifImg)
	}
	return layout.Dimensions{}
}

func (m *Message) loadGif() (*Gif, error) {
	var filePath = m.Filename
	if !m.isMe() {
		filePath = core.GetFilePath(m.Sender, m.Filename)
	}
	gif, err := LoadGif(filePath, false)
	return gif, err
}

func (m *Message) loadImage() (*image.Image, error) {
	var filePath = m.Filename
	if !m.isMe() {
		filePath = core.GetFilePath(m.Sender, m.Filename)
	}
	img, err := LoadImage(filePath, false)
	return img, err
}

func (m *Message) drawGif(gtx layout.Context, gif *Gif) layout.Dimensions {
	v := float32(gtx.Constraints.Max.X) * 0.382
	gtx.Constraints.Min.X = int(v)
	macro := op.Record(gtx.Ops)
	d := gif.Layout(gtx)
	call := macro.Stop()
	m.drawBorder(gtx, d, call)
	return d
}

func (m *Message) drawBrokenImage(gtx layout.Context) layout.Dimensions {
	m.imageBroken = true
	v := float32(gtx.Constraints.Max.X) * 0.382
	gtx.Constraints.Min.X = int(v)
	macro := op.Record(gtx.Ops)
	d := imageBrokenIcon.Layout(gtx, m.Theme.ContrastFg)
	call := macro.Stop()
	m.drawBorder(gtx, d, call)
	return d
}

func (m *Message) drawImage(gtx layout.Context, img image.Image) layout.Dimensions {
	v := float32(gtx.Constraints.Max.X) * 0.382
	dx := img.Bounds().Dx()
	dy := img.Bounds().Dy()
	var point image.Point
	if dx < dy {
		point = image.Point{X: int(v), Y: int(float32(dy) / float32(dx) * v)}
	} else {
		point = image.Point{X: int(float32(dx) / float32(dy) * v), Y: int(v)}
	}
	gtx.Constraints.Max = point
	macro := op.Record(gtx.Ops)
	imgOps := paint.NewImageOp(img)
	d := widget.Image{Src: imgOps, Fit: widget.Contain, Position: layout.Center, Scale: 0}.Layout(gtx)
	call := macro.Stop()
	m.drawBorder(gtx, d, call)
	return d
}

func (m *Message) drawBorder(gtx layout.Context, d layout.Dimensions, call op.CallOp) {
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
	defer pointer.PassOp{}.Push(gtx.Ops).Pop()
	m.InteractiveSpan.Layout(gtx)
	component.Rect{Color: bgColor, Size: d.Size}.Layout(gtx)
	// draw text
	call.Add(gtx.Ops)
}

func (m *Message) drawState(gtx layout.Context) layout.Dimensions {
	if m.isMe() {
		iconColor := m.ContrastBg
		var icon *widget.Icon
		switch m.State {
		case Stateless:
			loader := material.LoaderStyle{Color: m.ContrastBg}
			return loader.Layout(gtx)
		case Failed:
			icon = alertErrorIcon
			iconColor = color.NRGBA(colornames.Red500)
		case Sent:
			icon = actionDoneIcon
		case Read:
			icon = actionDoneAllIcon
		}
		return icon.Layout(gtx, iconColor)
	}
	return layout.Dimensions{}
}

func (m *Message) drawName(gtx layout.Context) layout.Dimensions {
	timeVal := m.CreatedAt
	timeMsg := timeVal.Local().Format("01/02, 15:04")
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
	l.getFocusAndResetIconStackIfClicked(gtx)
	// We visualize the text using a list where each paragraph is a separate item.
	dimensions := l.Clickable.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return l.List.Layout(gtx, len(l.Messages), func(gtx layout.Context, index int) layout.Dimensions {
			return l.Messages[index].Layout(gtx)
		})
	})
	l.scrollToEndIfFirstAndLastItemVisible()
	return dimensions
}

func (l *MessageList) scrollToEndIfFirstAndLastItemVisible() {
	// at end of list
	if !l.Position.BeforeEnd {
		// if at end and first item visible, scroll to end
		// or else, enable scroll by unset ScrollToEnd
		l.ScrollToEnd = l.Position.First == 0
		l.Position.BeforeEnd = true
		// received new message, not displayed
		if l.Position.First+l.Position.Count < len(l.Messages) {
			l.ScrollToEnd = true
		}
	}
}

func (l *MessageList) getFocusAndResetIconStackIfClicked(gtx layout.Context) {
	if l.Clicked(gtx) {
		for i := l.Position.First; i < l.Position.First+l.Position.Count; i++ {
			if !l.Messages[i].hovering {
				gtx.Execute(key.FocusCmd{Tag: l})
				l.Messages[i].longPressed = false
			}
		}
		iconStackAnimation.Disappear(gtx.Now)
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
		if _, submit = ev.(ui.SubmitEvent); submit {
			break
		}
		if !ok {
			break
		}
	}
	return submit
}

var MessageBox = make(chan *Message, 10)
