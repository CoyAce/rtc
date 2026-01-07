package view

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
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
	"rtc/internal/audio"
	ui "rtc/ui/layout"
	text "rtc/ui/layout/component"
	mt "rtc/ui/layout/material"
	"strings"
	"sync"
	"time"

	"gioui.org/f32"
	"gioui.org/font"
	"gioui.org/gesture"
	"gioui.org/io/clipboard"
	"gioui.org/io/event"
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
	"github.com/CoyAce/opus/ogg"
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
	audio.StreamConfig
	buffer []*Message
	ready  chan struct{}
	lock   sync.Mutex
}

func (k *MessageKeeper) Loop() {
	k.ready = make(chan struct{}, 1)
	flushFreq := 5 * time.Minute
	timer := time.NewTimer(flushFreq)
	for {
		timer.Reset(flushFreq)
		select {
		case msg := <-k.MessageChannel:
			k.lock.Lock()
			k.buffer = append(k.buffer, msg)
			k.lock.Unlock()
		case <-timer.C:
			k.ready <- struct{}{}
		case <-k.ready:
			if len(k.buffer) == 0 {
				continue
			}
			k.Append()
		}
	}
}

func (k *MessageKeeper) Append() {
	filePath := core.GetDataPath("message.log")
	file, err := os.OpenFile(filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Printf("Couldn't open file: %v", err)
	}
	defer file.Close()
	k.lock.Lock()
	defer k.lock.Unlock()
	for _, msg := range k.buffer {
		k.writeJson(file, msg)
	}
	k.buffer = k.buffer[:0]
}

func (k *MessageKeeper) writeJson(file *os.File, msg *Message) {
	s, err := json.Marshal(msg)
	if err != nil {
		log.Printf("Couldn't marshall message: %v", err)
		return
	}
	_, err = file.WriteString(string(s) + "\n")
	if err != nil {
		log.Printf("Couldn't write to file: %v", err)
	}
}

func (k *MessageKeeper) Messages() []*Message {
	filePath := core.GetDataPath("message.log")
	f, err := os.Open(filePath)
	if err != nil {
		log.Printf("Couldn't open file: %v", err)
		return []*Message{}
	}
	ret := make([]*Message, 0, 32)
	s := bufio.NewScanner(f)
	for s.Scan() {
		var msg Message
		line := s.Bytes()
		err = json.Unmarshal(line, &msg)
		if err != nil {
			log.Printf("Couldn't unmarshall message: %v", err)
		}
		msg.TextControl = NewTextControl(msg.Text)
		msg.Theme = fonts.DefaultTheme
		if msg.State == Stateless {
			msg.State = Failed
		}
		if msg.Type == Voice {
			msg.StreamConfig = k.StreamConfig
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
	Voice
)

// LongPressDuration is the default duration of a long press gesture.
// Override this variable to change the detection threshold.
var LongPressDuration = 250 * time.Millisecond

// EventType describes a kind of iteraction with rich text.
type EventType uint8

const (
	Hover EventType = iota
	Unhover
	LongPress
	Click
	LongPressRelease
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

func (i *InteractiveSpan) Clicked(gtx layout.Context) bool {
	for {
		e, ok := i.Update(gtx)
		if !ok {
			break
		}
		if e.Type == Click || e.Type == LongPressRelease {
			return true
		}
	}
	return false
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
				return Event{Type: LongPressRelease}, true
			}
			i.longPressed = false
			return Event{Type: Click, ClickData: e}, true
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
	for {
		e, ok := gtx.Event(
			key.FocusFilter{Target: i},
		)
		if !ok {
			break
		}
		switch e := e.(type) {
		case key.FocusEvent:
			if !e.Focus && !i.hovering {
				i.longPressed = false
			}
		}
	}
	defer clip.Rect{Max: gtx.Constraints.Max}.Push(gtx.Ops).Pop()

	pointer.CursorPointer.Add(gtx.Ops)
	i.click.Add(gtx.Ops)
	event.Op(gtx.Ops, i)
	return layout.Dimensions{}
}

type Message struct {
	State
	*material.Theme `json:"-"`
	InteractiveSpan `json:"-"`
	MediaControl    `json:"-"`
	FileControl     `json:"-"`
	TextControl     `json:"-"`
	Filename        string
	Size            int
	Type            MessageType
	UUID            string
	Text            string
	Sender          string
	CreatedAt       time.Time
}

type TextControl struct {
	Editor     *ui.Editor
	copyButton widget.Clickable
}

func NewTextControl(text string) TextControl {
	ed := ui.Editor{ReadOnly: true}
	ed.SetText(text)
	return TextControl{Editor: &ed}
}

type FileControl struct {
	downloadButton widget.Clickable
	imageBroken    bool
}

type MediaControl struct {
	audio.StreamConfig `json:"-"`
	playButton         widget.Clickable
	pauseButton        widget.Clickable
	cancel             context.CancelFunc
	playing            bool
	startAt            time.Time
}

type MessageEditor struct {
	*material.Theme
	InteractiveSpan
	InputField   *text.TextField
	submitButton widget.Clickable
	EditorOperator
	ExpandButton
}

type EditorOperator struct {
	cutButton   widget.Clickable
	copyButton  widget.Clickable
	pasteButton widget.Clickable
}

func (m *Message) Layout(gtx layout.Context) (d layout.Dimensions) {
	if m.Type == Text && m.Text == "" {
		return d
	}
	if (m.Type == Image || m.Type == Voice) && m.Filename == "" {
		return d
	}

	margins := layout.Inset{Top: unit.Dp(24), Bottom: unit.Dp(0), Left: unit.Dp(8), Right: unit.Dp(8)}
	return margins.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		flex := layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle, Spacing: layout.SpaceEnd}
		message := []layout.FlexChild{
			layout.Rigid(layout.Spacer{Width: unit.Dp(2)}.Layout),
			layout.Rigid(m.drawMessage),
			layout.Rigid(layout.Spacer{Width: unit.Dp(2)}.Layout),
		}
		var avatar layout.FlexChild
		if m.isMe() {
			flex.Spacing = layout.SpaceStart
			avatar = layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return m.drawAvatar(gtx, m.UUID)
			})
			message = append(message, avatar)
		} else {
			avatar = layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return m.drawAvatar(gtx, m.Sender)
			})
			message = append([]layout.FlexChild{avatar}, message...)
		}
		return flex.Layout(gtx, message...)
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
	m.processTextCopy(gtx, m.Text)
	m.processFileSave(gtx, m.FilePath())
	m.getFocusIfClickedToEnableFocusLostEvent(gtx)
	flex := layout.Flex{Axis: layout.Vertical, Alignment: layout.Start}
	if m.isMe() {
		flex.Alignment = layout.End
	}
	return flex.Layout(gtx,
		layout.Rigid(m.drawName),
		// state and message
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			flex := layout.Flex{Spacing: layout.SpaceEnd, Alignment: layout.Middle}
			contents := []layout.FlexChild{
				layout.Rigid(layout.Spacer{Width: unit.Dp(4)}.Layout),
				layout.Rigid(m.drawContent),
				layout.Rigid(layout.Spacer{Width: unit.Dp(4)}.Layout),
			}
			stateOrOperationBar := layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				if m.operationNeeded() {
					return m.drawOperation(gtx)
				}
				return m.drawState(gtx)
			})
			if m.isMe() {
				contents = append([]layout.FlexChild{stateOrOperationBar}, contents...)
				flex.Spacing = layout.SpaceStart
			} else {
				contents = append(contents, stateOrOperationBar)
			}
			return flex.Layout(gtx, contents...)
		}),
	)
}

func (m *Message) getFocusIfClickedToEnableFocusLostEvent(gtx layout.Context) {
	if m.Clicked(gtx) {
		gtx.Execute(key.FocusCmd{Tag: &m.InteractiveSpan})
	}
}

func (m *Message) operationNeeded() bool {
	return m.Type != Voice && (m.longPressed || m.Editor != nil && m.Editor.SelectionLen() > 0)
}

func (m *TextControl) processTextCopy(gtx layout.Context, textForCopy string) {
	if m.copyButton.Clicked(gtx) {
		if m.Editor != nil && m.Editor.SelectionLen() > 0 {
			textForCopy = m.Editor.SelectedText()
		}
		gtx.Execute(clipboard.WriteCmd{Type: "application/text", Data: io.NopCloser(strings.NewReader(textForCopy))})
	}
	if m.Editor != nil && !gtx.Focused(m.Editor) {
		m.Editor.ClearSelection()
	}
}

func (m *FileControl) processFileSave(gtx layout.Context, filePath string) {
	if !m.downloadButton.Clicked(gtx) {
		return
	}
	go func() {
		if filePath == "" {
			return
		}
		w, err := DefaultPicker.CreateFile(filepath.Base(filePath))
		if err != nil {
			log.Printf("Couldn't create file for %s: %s", filePath, err)
			return
		}
		defer w.Close()
		r, err := os.Open(filePath)
		if err != nil {
			log.Printf("Couldn't open file for %s: %s", filePath, err)
			return
		}
		defer r.Close()
		_, err = io.Copy(w, r)
		if err != nil {
			log.Printf("Couldn't save file for %s: %s", filePath, err)
			return
		}
	}()
}

func (m *Message) FilePath() string {
	var filePath = m.Filename
	if !m.isMe() {
		filePath = core.GetPath(m.Sender, m.Filename)
	}
	return filePath
}

func (m *Message) drawOperation(gtx layout.Context) layout.Dimensions {
	if m.imageBroken {
		return layout.Dimensions{}
	}
	switch m.Type {
	case Text:
		return m.copyButton.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return contentCopyIcon.Layout(gtx, m.ContrastBg)
		})
	case Image:
		fallthrough
	case GIF:
		return m.downloadButton.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
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
			return mt.Editor(m.Theme, m.Editor, "hint").Layout(gtx)
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
	case Voice:
		if m.Filename == "" {
			return layout.Dimensions{}
		}
		return m.drawVoice(gtx)
	}
	return layout.Dimensions{}
}

func (m *Message) drawVoice(gtx layout.Context) layout.Dimensions {
	v := float32(gtx.Constraints.Max.X) * 0.382
	gtx.Constraints.Min.X = int(float32(gtx.Constraints.Max.X) * 0.618)
	gtx.Constraints.Min.Y = int(v * 0.382)
	macro := op.Record(gtx.Ops)
	d := m.drawAudioControl(gtx, m.Filename, v, m.ContrastBg)
	layout.Stack{Alignment: layout.E}.Layout(gtx,
		layout.Stacked(func(gtx layout.Context) layout.Dimensions {
			left := m.getLeftDuration()
			label := material.Label(m.Theme, m.Theme.TextSize*0.70, fmt.Sprintf("%.1fs", left.Seconds()))
			label.Font.Weight = font.Bold
			label.Color = m.Theme.ContrastFg
			return layout.UniformInset(unit.Dp(8)).Layout(gtx, label.Layout)
		}))
	call := macro.Stop()
	m.drawBorder(gtx, d, call)
	return d
}

func (m *MediaControl) drawAudioControl(gtx layout.Context, filename string, size float32, fgColor color.NRGBA) layout.Dimensions {
	return layout.Flex{Spacing: layout.SpaceAround, Alignment: layout.Middle}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			btn := &m.playButton
			icon := playIcon
			if m.playButton.Clicked(gtx) {
				m.playing = true
				m.startAt = time.Now()
				m.playAudioAsync(filename)
			}
			if m.pauseButton.Clicked(gtx) {
				m.playing = false
				m.cancel()
			}
			if m.playing {
				btn = &m.pauseButton
				icon = pauseIcon
			}
			return btn.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				gtx.Constraints.Min.X = int(size * 0.382)
				return icon.Layout(gtx, fgColor)
			})
		}),
	)
}

func (m *Message) getLeftDuration() time.Duration {
	size := time.Duration(m.Size) * time.Millisecond
	if !m.playing {
		return size
	}
	elapsed := time.Since(m.startAt)
	left := size - elapsed
	if left < 0 {
		return 0
	}
	return left
}

func (m *MediaControl) playAudioAsync(filename string) {
	go func() {
		file, err := os.Open(filename)
		if err != nil {
			log.Printf("open file failed, %v", err)
			return
		}
		pcm, err := ogg.Decode(file)
		if err != nil {
			log.Printf("decode file failed, %v", err)
		}
		reader := bytes.NewReader(pcm)
		var ctx context.Context
		ctx, m.cancel = context.WithCancel(context.Background())
		if err := audio.Playback(ctx, reader, m.StreamConfig); err != nil && !errors.Is(err, io.EOF) {
			log.Printf("audio playback: %w", err)
		}
		m.playing = false
	}()
}

func (m *Message) loadGif() (*Gif, error) {
	gif, err := LoadGif(m.FilePath(), false)
	return gif, err
}

func (m *Message) loadImage() (*image.Image, error) {
	img, err := LoadImage(m.FilePath(), false)
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
		gtx.Execute(key.FocusCmd{Tag: &l.Clickable})
		iconStackAnimation.Disappear(gtx.Now)
	}
}

func (e *MessageEditor) Layout(gtx layout.Context) layout.Dimensions {
	e.processTextCut(gtx)
	e.processTextCopy(gtx)
	e.processTextPaste(gtx)
	if e.operationBarNeeded(gtx) {
		e.EditorOperator.Layout(gtx)
	}
	if !gtx.Focused(&e.InputField.Editor) {
		e.hideOperationBar()
	}
	defer clip.Rect(image.Rectangle{Max: gtx.Constraints.Max}).Push(gtx.Ops).Pop()
	defer pointer.PassOp{}.Push(gtx.Ops).Pop()
	e.InteractiveSpan.Layout(gtx)
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
			layout.Rigid(e.ExpandButton.Layout),
		)
	})
}

func (e *MessageEditor) operationBarNeeded(gtx layout.Context) bool {
	return e.longPressed && gtx.Focused(&e.InputField.Editor) || e.InputField.SelectionLen() > 0
}

func (e *MessageEditor) processTextCut(gtx layout.Context) {
	if e.cutButton.Clicked(gtx) {
		if e.InputField.Editor.SelectionLen() > 0 {
			textForCopy := e.InputField.Editor.SelectedText()
			gtx.Execute(clipboard.WriteCmd{Type: "application/text", Data: io.NopCloser(strings.NewReader(textForCopy))})
			e.InputField.Editor.Delete(1)
		}
		e.hideOperationBar()
	}
}

func (e *MessageEditor) processTextPaste(gtx layout.Context) {
	if e.pasteButton.Clicked(gtx) {
		if e.InputField.Editor.SelectionLen() > 0 {
			e.InputField.Editor.Delete(1)
		}
		gtx.Execute(clipboard.ReadCmd{Tag: &e.InputField.Editor})
		e.hideOperationBar()
	}
}

func (e *MessageEditor) processTextCopy(gtx layout.Context) {
	if e.copyButton.Clicked(gtx) {
		textForCopy := e.InputField.Text()
		if e.InputField.Editor.SelectionLen() > 0 {
			textForCopy = e.InputField.Editor.SelectedText()
		}
		gtx.Execute(clipboard.WriteCmd{Type: "application/text", Data: io.NopCloser(strings.NewReader(textForCopy))})
		e.hideOperationBar()
	}
}

func (e *MessageEditor) hideOperationBar() {
	e.longPressed = false
	e.InputField.Editor.ClearSelection()
}

func (e *EditorOperator) Layout(gtx layout.Context) {
	defer op.Offset(image.Point{Y: -gtx.Dp(24)}).Push(gtx.Ops).Pop()
	macro := op.Record(gtx.Ops)
	icons := layout.UniformInset(unit.Dp(4)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Stack{Alignment: layout.Center}.Layout(gtx,
			layout.Stacked(func(gtx layout.Context) layout.Dimensions {
				offset := image.Pt(-gtx.Dp(82), 0)
				op.Offset(offset).Add(gtx.Ops)
				return e.cutButton.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return contentCutIcon.Layout(gtx, fonts.DefaultTheme.ContrastFg)
				})
			}),
			layout.Stacked(func(gtx layout.Context) layout.Dimensions {
				offset := image.Pt(-gtx.Dp(54), 0)
				op.Offset(offset).Add(gtx.Ops)
				return e.copyButton.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return contentCopyIcon.Layout(gtx, fonts.DefaultTheme.ContrastFg)
				})
			}),
			layout.Stacked(func(gtx layout.Context) layout.Dimensions {
				offset := image.Pt(-gtx.Dp(26), 0)
				op.Offset(offset).Add(gtx.Ops)
				return e.pasteButton.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return contentPasteIcon.Layout(gtx, fonts.DefaultTheme.ContrastFg)
				})
			}),
		)
	})
	call := macro.Stop()
	e.drawBorder(gtx, icons, call)
}

func (e *EditorOperator) drawBorder(gtx layout.Context, icons layout.Dimensions, call op.CallOp) {
	bgColor := fonts.DefaultTheme.ContrastBg
	bgColor.A = 192
	// https://pomax.github.io/bezierinfo/#circles_cubic.
	const q = 4 * (math.Sqrt2 - 1) / 3
	const iq = 1 - q
	midX := float32(icons.Size.X)/2 - float32(gtx.Dp(54))
	minX := midX - float32(gtx.Dp(24))*float32(1.5) - float32(gtx.Dp(4*2))
	maxX := midX + float32(gtx.Dp(24))*float32(1.5) + float32(gtx.Dp(4*2))
	minY := float32(0)
	maxY := float32(gtx.Dp(32))
	se, sw, nw, ne := float32(gtx.Dp(4)), float32(gtx.Dp(4)), float32(gtx.Dp(4)), float32(gtx.Dp(4))
	triangleLegHalfSize := float32(gtx.Dp(3))
	perpendicular := float32(float64(triangleLegHalfSize*2) * math.Sin(math.Pi/3))

	p := clip.Path{}
	p.Begin(gtx.Ops)
	p.MoveTo(f32.Point{X: minX + nw, Y: minY})
	p.LineTo(f32.Point{X: maxX - ne, Y: minY}) // N
	p.CubeTo(f32.Point{X: maxX - ne*iq, Y: minY}, // NE
		f32.Point{X: maxX, Y: minY + ne*iq},
		f32.Point{X: maxX, Y: minY + ne})
	p.LineTo(f32.Point{X: maxX, Y: maxY - se}) // E
	p.CubeTo(f32.Point{X: maxX, Y: maxY - se*iq}, // SE
		f32.Point{X: maxX - se*iq, Y: maxY},
		f32.Point{X: maxX - se, Y: maxY})
	p.LineTo(f32.Point{X: midX + triangleLegHalfSize, Y: maxY}) // S
	p.LineTo(f32.Point{X: midX, Y: maxY + perpendicular})       // S
	p.LineTo(f32.Point{X: midX - triangleLegHalfSize, Y: maxY}) // S
	p.LineTo(f32.Point{X: minX + sw, Y: maxY})                  // S
	p.CubeTo(f32.Point{X: minX + sw*iq, Y: maxY}, // SW
		f32.Point{X: minX, Y: maxY - sw*iq},
		f32.Point{X: minX, Y: maxY - sw})
	p.LineTo(f32.Point{X: minX, Y: minY + nw}) // W
	p.CubeTo(f32.Point{X: minX, Y: minY + nw*iq}, // NW
		f32.Point{X: minX + nw*iq, Y: minY},
		f32.Point{X: minX + nw, Y: minY})

	path := p.End()

	defer clip.Outline{Path: path}.Op().Push(gtx.Ops).Pop()
	paint.Fill(gtx.Ops, bgColor)
	pointer.CursorPointer.Add(gtx.Ops)
	call.Add(gtx.Ops)
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
