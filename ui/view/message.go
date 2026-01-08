package view

import (
	"bytes"
	"context"
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
	"rtc/core"
	"rtc/internal/audio"
	app "rtc/ui/layout"
	mt "rtc/ui/layout/material"
	"strings"
	"time"

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
	Press
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
			return Event{Type: Press, ClickData: e}, true
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
	MediaControl
	*material.Theme `json:"-"`
	InteractiveSpan `json:"-"`
	FileControl     `json:"-"`
	TextControl     `json:"-"`
	Filename        string
	Size            uint64
	Type            MessageType
	UUID            string
	Text            string
	Sender          string
	CreatedAt       time.Time
}

type TextControl struct {
	Editor     *app.Editor
	copyButton widget.Clickable
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

func NewTextControl(text string) TextControl {
	ed := app.Editor{ReadOnly: true}
	ed.SetText(text)
	return TextControl{Editor: &ed}
}

type FileControl struct {
	downloadButton widget.Clickable
	imageBroken    bool
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

func (m *FileControl) loadGif(filepath string) (*Gif, error) {
	gif, err := LoadGif(filepath, false)
	return gif, err
}

func (m *FileControl) loadImage(filepath string) (*image.Image, error) {
	img, err := LoadImage(filepath, false)
	return img, err
}

type MediaControl struct {
	audio.StreamConfig `json:"-"`
	Duration           uint64
	playButton         widget.Clickable
	animation          *component.Progress
	pauseButton        widget.Clickable
	cancel             context.CancelFunc
}

func (m *MediaControl) Layout(gtx layout.Context, filePath string, fgColor color.NRGBA) layout.Dimensions {
	return layout.Flex{Spacing: layout.SpaceAround, Alignment: layout.Middle}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			btn := &m.playButton
			icon := playIcon
			if m.animation == nil {
				m.animation = &component.Progress{}
			}
			if m.playButton.Clicked(gtx) {
				m.animation.Start(gtx.Now, component.Reverse, time.Duration(m.Duration)*time.Millisecond)
				m.playAudioAsync(filePath)
			}
			if m.pauseButton.Clicked(gtx) {
				m.animation.Stop()
				if m.cancel != nil {
					m.cancel()
				}
			}
			if m.animation.Started() {
				gtx.Execute(op.InvalidateCmd{})
				btn = &m.pauseButton
				icon = pauseIcon
			}
			m.animation.Update(gtx.Now)
			return btn.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				gtx.Constraints.Min.X = gtx.Constraints.Min.Y
				return icon.Layout(gtx, fgColor)
			})
		}),
	)
}

func (m *MediaControl) getLeftDuration() time.Duration {
	size := time.Duration(m.Duration) * time.Millisecond
	if !m.animation.Started() {
		return size
	}
	return time.Duration(float32(size) * m.animation.Progress())
}

func (m *MediaControl) playAudioAsync(filePath string) {
	go func() {
		file, err := os.Open(filePath)
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
		m.animation.Stop()
	}()
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
	avatar := AvatarCache.LoadOrElseNew(uuid)
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
		layout.Rigid(m.drawStateAndContent),
	)
}

func (m *Message) getFocusIfClickedToEnableFocusLostEvent(gtx layout.Context) {
	if m.Clicked(gtx) {
		gtx.Execute(key.FocusCmd{Tag: &m.InteractiveSpan})
	}
}

func (m *Message) drawStateAndContent(gtx layout.Context) layout.Dimensions {
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
}

func (m *Message) operationNeeded() bool {
	return m.longPressed || m.Editor != nil && m.Editor.SelectionLen() > 0
}

func (m *Message) FilePath() string {
	if !m.isMe() {
		return core.GetPath(m.Sender, m.Filename)
	}
	return core.GetPath(m.UUID, m.Filename)
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
	case Image, Voice:
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
		img, err := m.loadImage(m.FilePath())
		if err != nil || img == nil || img == &assets.AppIconImage {
			return m.drawBrokenImage(gtx)
		}
		return m.drawImage(gtx, *img)
	case GIF:
		if m.Filename == "" {
			return layout.Dimensions{}
		}
		gifImg, err := m.loadGif(m.FilePath())
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
	d := m.MediaControl.Layout(gtx, m.FilePath(), m.ContrastBg)
	layout.Stack{Alignment: layout.E}.Layout(gtx,
		layout.Stacked(func(gtx layout.Context) layout.Dimensions {
			left := m.getLeftDuration()
			label := material.Label(m.Theme, m.TextSize*0.70, fmt.Sprintf("%.1fs", left.Seconds()))
			label.Font.Weight = font.Bold
			label.Color = m.ContrastFg
			return layout.UniformInset(unit.Dp(8)).Layout(gtx, label.Layout)
		}))
	call := macro.Stop()
	m.drawBorder(gtx, d, call)
	return d
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
	loc, _ := time.LoadLocation("Asia/Shanghai")
	timeMsg := timeVal.In(loc).Format("01/02 15:04")
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

var MessageBox = make(chan *Message, 10)
