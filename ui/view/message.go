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
	"rtc/assets/icons"
	"rtc/internal/audio"
	"rtc/ui/native"
	"runtime"
	"strconv"
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
	"github.com/CoyAce/wi"
	"golang.org/x/exp/shiny/materialdesign/colornames"
)

// ShanghaiLoc 上海时区（UTC+8）
var ShanghaiLoc = time.FixedZone("Asia/Shanghai", 8*60*60) // 8小时 * 60分钟 * 60秒

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
	File
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
			if !e.Focus {
				i.pressing = false
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
	MessageStyle    `json:"-"`
	InteractiveSpan `json:"-"`
	FileControl
	TextControl
	MessageType
	Contacts
	CreatedAt time.Time
}

type MessageStyle struct {
	*material.Theme    `json:"-"`
	layout.Constraints `json:"-"`
}

func (m *MessageStyle) getBaseWidth() float32 {
	return float32(m.Max.X) * 0.618
}

func (m *MessageStyle) getReverseBaseWidth() float32 {
	return float32(m.Max.X) * 0.382
}

type Contacts struct {
	UUID   string
	Sender string
}

func FromSender(sender string) Contacts {
	return Contacts{UUID: wi.DefaultClient.FullID(), Sender: sender}
}

func FromMyself() Contacts {
	return FromSender(wi.DefaultClient.FullID())
}

type TextControl struct {
	Text       string
	Editor     *widget.Editor `json:"-"`
	copyButton widget.Clickable
}

func (m *TextControl) processTextCopy(gtx layout.Context, textForCopy string) {
	if m.copyButton.Clicked(gtx) {
		if m.Editor != nil && m.Editor.SelectionLen() > 0 {
			textForCopy = m.Editor.SelectedText()
		}
		gtx.Execute(clipboard.WriteCmd{Type: "application/text", Data: io.NopCloser(strings.NewReader(textForCopy))})
	}
	if m.Editor != nil && !gtx.Focused(m.Editor) && m.Editor.SelectionLen() > 0 {
		m.Editor.ClearSelection()
	}
}

func NewTextControl(text string) TextControl {
	ed := widget.Editor{ReadOnly: true}
	ed.SetText(text)
	return TextControl{Text: text, Editor: &ed}
}

type Mime byte

const (
	Unknown Mime = iota
	Picture
	Music
	Video
	Ebook
	Apk
)

func NewMine(filename string) Mime {
	switch filepath.Ext(filename) {
	case ".apk":
		return Apk
	case ".jpg", ".jpeg", ".png", ".webp", ".gif", ".svg":
		return Picture
	case ".epub", ".pdf":
		return Ebook
	case ".wav", ".mp3", ".flac", ".ogg", ".opus":
		return Music
	case ".mp4", ".avi", ".mov", ".webm":
		return Video
	default:
		return Unknown
	}
}

type FileControl struct {
	Filename string
	FileId   uint32
	Path     string
	Size     uint64
	Mime
	progress       int
	speed          int
	saveButton     widget.Clickable
	downloadButton widget.Clickable
	browseButton   widget.Clickable
	imageBroken    bool
}

func (f *FileControl) downloading() bool {
	return f.progress > 0 && f.progress < 100
}

func (f *FileControl) downloaded() bool {
	return f.progress == 100
}

func (f *FileControl) processFileDownload(gtx layout.Context, sender string) {
	if !f.downloadButton.Clicked(gtx) {
		return
	}
	log.Printf("downloading...")
	go func() {
		err := wi.DefaultClient.SubscribeFile(f.FileId, sender,
			func(p int, s int) {
				f.updateProgress(p)
				f.updateSpeed(s)
				InvalidateRequest <- struct{}{}
			})
		if err != nil {
			log.Printf("Subsrcibe file failed: %v", err)
		}
	}()
}

func (f *FileControl) processFileBrowse(gtx layout.Context, path string) {
	if !f.browseButton.Clicked(gtx) {
		return
	}
	log.Printf("browsing file...")
	go func() {
		err := OpenInFinder(path)
		if err != nil {
			log.Printf("Open in finder failed: %v", err)
		}
	}()
}

func (f *FileControl) processPhotoSave(gtx layout.Context, path string) {
	if !f.saveButton.Clicked(gtx) || path == "" {
		return
	}
	go func() {
		err := native.Tool.SavePhoto(path)
		if err != nil {
			log.Printf("Save photo failed: %v", err)
			return
		}
		log.Printf("send hint request")
		HintRequest <- "✅完成"
	}()
}

func (f *FileControl) processFileSave(gtx layout.Context, path string) {
	if !f.saveButton.Clicked(gtx) || path == "" {
		return
	}
	go func() {
		w, err := Picker.CreateFile(f.Filename)
		if err != nil {
			log.Printf("Create file %s failed: %s", path, err)
			return
		}
		defer w.Close()
		r, err := Open(path)
		if err != nil {
			log.Printf("Open file %s failed: %s", path, err)
			return
		}
		defer r.Close()
		_, err = io.Copy(w, r)
		if err != nil {
			log.Printf("Save file %s failed: %s", path, err)
			return
		}
		HintRequest <- "✅完成"
	}()
}

func (f *FileControl) updateProgress(p int) {
	f.progress = p
}

func (f *FileControl) updateSpeed(s int) {
	f.speed = s
}

func (f *FileControl) loadGif(filepath string) *Gif {
	return LoadGif(filepath, false)
}

func (f *FileControl) loadImage(filepath string) *image.Image {
	return LoadImage(filepath, false)
}

func (f *FileControl) Layout(gtx layout.Context, theme *material.Theme) layout.Dimensions {
	title, progress := f.compose(theme)
	return layout.Flex{WeightSum: 1.0, Alignment: layout.Middle, Spacing: layout.SpaceEvenly}.Layout(gtx,
		layout.Rigid(f.drawIcon(theme)),
		layout.Flexed(0.618, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical, Spacing: layout.SpaceAround}.Layout(gtx,
				title...,
			)
		}),
		layout.Flexed(1-0.618, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Spacing: layout.SpaceSides}.Layout(gtx, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Vertical, Spacing: layout.SpaceAround}.Layout(gtx,
					progress...,
				)
			}))
		}),
	)
}

func (f *FileControl) compose(theme *material.Theme) ([]layout.FlexChild, []layout.FlexChild) {
	title := []layout.FlexChild{layout.Rigid(f.drawFilename(theme))}
	progress := []layout.FlexChild{layout.Rigid(f.drawProgress(theme)), layout.Rigid(f.drawSpeed(theme))}
	sizeWidget := layout.Rigid(f.drawSize(theme))
	if f.isLongName() {
		return title, append([]layout.FlexChild{sizeWidget}, progress...)
	}
	return append(title, sizeWidget), progress
}

func (f *FileControl) isLongName() bool {
	return len(f.Filename) >= 25
}

func (f *FileControl) drawSpeed(theme *material.Theme) func(gtx layout.Context) layout.Dimensions {
	return func(gtx layout.Context) layout.Dimensions {
		label := material.Label(theme, theme.TextSize, f.getSpeed())
		label.Color = theme.ContrastFg
		gtx.Constraints.Min.X = 0
		margins := layout.Inset{Top: unit.Dp(4), Bottom: unit.Dp(4)}
		return margins.Layout(gtx, label.Layout)
	}
}

func (f *FileControl) getSpeed() string {
	if f.speed == 0 {
		return "-"
	}
	return f.toHumanReadable(float32(f.speed)) + "/s"
}

func (f *FileControl) drawIcon(theme *material.Theme) func(gtx layout.Context) layout.Dimensions {
	return func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Min.X = gtx.Constraints.Min.Y
		return f.getIconByMimeType().Layout(gtx, theme.ContrastFg)
	}
}

func (f *FileControl) drawProgress(theme *material.Theme) func(gtx layout.Context) layout.Dimensions {
	return func(gtx layout.Context) layout.Dimensions {
		label := material.Label(theme, theme.TextSize, f.getProgress())
		label.Color = theme.ContrastFg
		gtx.Constraints.Min.X = 0
		margins := layout.Inset{Top: unit.Dp(4), Bottom: unit.Dp(4)}
		return margins.Layout(gtx, label.Layout)
	}
}

func (f *FileControl) getProgress() string {
	if f.Path != "" {
		return "-"
	}
	if f.progress == 0 {
		return "未下载"
	}
	return strconv.Itoa(f.progress) + "%"
}

func (f *FileControl) drawSize(theme *material.Theme) func(gtx layout.Context) layout.Dimensions {
	return func(gtx layout.Context) layout.Dimensions {
		label := material.Label(theme, theme.TextSize, f.toHumanReadable(float32(f.Size)))
		label.Color = theme.ContrastFg
		gtx.Constraints.Min.X = 0
		margins := layout.Inset{Top: unit.Dp(4), Bottom: unit.Dp(4)}
		return margins.Layout(gtx, label.Layout)
	}
}

func (f *FileControl) drawFilename(theme *material.Theme) func(layout.Context) layout.Dimensions {
	return func(gtx layout.Context) layout.Dimensions {
		label := material.Label(theme, theme.TextSize, f.Filename)
		label.Font.Weight = font.Bold
		label.Color = theme.ContrastFg
		gtx.Constraints.Min.X = 0
		margins := layout.Inset{Top: unit.Dp(4), Bottom: unit.Dp(4)}
		return margins.Layout(gtx, label.Layout)
	}
}

func (f *FileControl) getIconByMimeType() *widget.Icon {
	switch f.Mime {
	case Apk:
		return icons.ApkIcon
	case Picture:
		return icons.ImageIcon
	case Ebook:
		return icons.BookIcon
	case Music:
		return icons.MusicIcon
	case Video:
		return icons.VideoIcon
	default:
		return icons.UnknownIcon
	}
}

func (f *FileControl) toHumanReadable(v float32) string {
	suffix := "B"
	if v < 1024 {
		suffix = "B"
	} else if v < 1024*1024 {
		suffix = "KB"
		v /= 1024
	} else if v < 1024*1024*1024 {
		suffix = "MB"
		v /= 1024 * 1024
	}
	return fmt.Sprintf("%.1f %s", v, suffix)
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
			icon := icons.PlayIcon
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
				icon = icons.PauseIcon
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
		pcm, channels, err := ogg.Decode(file)
		if err != nil {
			log.Printf("decode file failed, %v", err)
		}
		m.StreamConfig.Channels = channels
		reader := bytes.NewReader(pcm)
		var ctx context.Context
		ctx, m.cancel = context.WithCancel(context.Background())
		if err := audio.Playback(ctx, reader, m.StreamConfig); err != nil && !errors.Is(err, io.EOF) {
			log.Printf("audio playback: %v", err)
		}
		m.animation.Stop()
	}()
}

func (m *Message) Layout(gtx layout.Context) (d layout.Dimensions) {
	if m.MessageType == Text && m.Text == "" {
		return d
	}
	if (m.MessageType == Image || m.MessageType == Voice) && m.fileNotExist() {
		return d
	}
	m.Constraints = gtx.Constraints

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
	m.processFileViewAndSave(gtx)
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

func (m *Message) processFileViewAndSave(gtx layout.Context) {
	if runtime.GOOS == "ios" && (m.MessageType == Image || m.MessageType == GIF) {
		m.processFileBrowse(gtx, m.OptimizedFilePath())
		m.processPhotoSave(gtx, m.OptimizedFilePath())
		return
	}
	switch m.MessageType {
	case Voice:
		m.processFileBrowse(gtx, m.FilePath())
		m.processFileSave(gtx, m.FilePath())
	case File:
		m.processFileDownload(gtx, m.Sender)
		fallthrough
	default:
		m.processFileBrowse(gtx, m.OptimizedFilePath())
		m.processFileSave(gtx, m.OptimizedFilePath())
	}
}

func (m *Message) getFocusIfClickedToEnableFocusLostEvent(gtx layout.Context) {
	for {
		e, ok := m.InteractiveSpan.Update(gtx)
		if !ok {
			break
		}
		if e.Type == LongPress {
			if m.TextSelected() {
				m.longPressed = false
			} else {
				gtx.Execute(key.FocusCmd{Tag: &m.InteractiveSpan})
			}
		}
		if e.Type == Press {
			gtx.Execute(key.FocusCmd{Tag: &m.InteractiveSpan})
		}
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
	if m.MessageType == File {
		if m.isMe() || m.downloading() {
			return false
		}
	}
	return m.longPressed || m.TextSelected()
}

func (m *Message) TextSelected() bool {
	return m.Editor != nil && m.Editor.SelectionLen() > 0
}

func (m *Message) FilePath() string {
	if !m.isMe() {
		return GetPath(m.Sender, m.Filename)
	}
	return GetPath(m.UUID, m.Filename)
}

// OptimizedFilePath return path for received file and user chosen file
func (m *Message) OptimizedFilePath() string {
	if m.isMe() {
		if runtime.GOOS == "ios" {
			return GetExternalDir() + filepath.Base(m.Path)
		}
		return m.Path
	}
	return m.FilePath()
}

func (m *Message) drawOperation(gtx layout.Context) layout.Dimensions {
	if m.imageBroken {
		return layout.Dimensions{}
	}
	switch m.MessageType {
	case Text:
		return m.drawCopyButton(gtx)
	case Image, Voice, GIF:
		return m.drawViewAndSave(gtx)
	case File:
		if m.downloaded() {
			return m.drawViewAndSave(gtx)
		}
		return m.drawCloudDownloadButton(gtx)
	}
	return layout.Dimensions{}
}

func (m *Message) drawCopyButton(gtx layout.Context) layout.Dimensions {
	return m.copyButton.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return icons.ContentCopyIcon.Layout(gtx, m.ContrastBg)
	})
}

func (m *Message) drawCloudDownloadButton(gtx layout.Context) layout.Dimensions {
	return m.downloadButton.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return icons.CloudDownloadIcon.Layout(gtx, m.ContrastBg)
	})
}

func (m *Message) drawViewAndSave(gtx layout.Context) layout.Dimensions {
	return layout.Flex{Axis: layout.Vertical, Alignment: layout.Middle}.Layout(gtx,
		layout.Rigid(m.drawViewButton),
		layout.Rigid(layout.Spacer{Height: unit.Dp(12)}.Layout),
		layout.Rigid(m.drawSaveButton),
	)
}

func (m *Message) drawViewButton(gtx layout.Context) layout.Dimensions {
	return m.browseButton.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return icons.BrowseIcon.Layout(gtx, m.ContrastBg)
	})
}

func (m *Message) drawSaveButton(gtx layout.Context) layout.Dimensions {
	return m.saveButton.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return icons.FileExportIcon.Layout(gtx, m.ContrastBg)
	})
}

func (m *Message) drawContent(gtx layout.Context) layout.Dimensions {
	if m.Text == "" && m.fileNotExist() {
		log.Printf("text: %v, name: %v, path: %v", m.Text, m.Filename, m.Path)
		return layout.Dimensions{}
	}
	switch m.MessageType {
	case Text:
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
		img := m.loadImage(m.OptimizedFilePath())
		if img == nil {
			return m.drawBrokenImage(gtx)
		}
		if *img == nil {
			return m.drawBlankBox()
		}
		return m.drawImage(gtx, *img)
	case GIF:
		gifImg := m.loadGif(m.OptimizedFilePath())
		if gifImg.GIF == nil {
			return m.drawBrokenImage(gtx)
		}
		return m.drawGif(gtx, gifImg)
	case Voice:
		return m.drawVoice(gtx)
	case File:
		return m.drawFile(gtx)
	}
	return layout.Dimensions{}
}

func (m *Message) drawFile(gtx layout.Context) layout.Dimensions {
	v := m.getReverseBaseWidth()
	gtx.Constraints.Min.X = int(m.getBaseWidth())
	gtx.Constraints.Min.Y = int(v * 0.382)
	gtx.Constraints.Max.X = gtx.Constraints.Min.X
	macro := op.Record(gtx.Ops)
	d := m.FileControl.Layout(gtx, m.Theme)
	call := macro.Stop()
	m.drawBorder(gtx, d, call)
	return d
}

func (m *Message) drawBlankBox() layout.Dimensions {
	v := int(m.getBaseWidth())
	return layout.Dimensions{Size: image.Pt(v, v)}
}

func (m *Message) fileNotExist() bool {
	return m.Filename == "" && m.Path == ""
}

func (m *Message) drawVoice(gtx layout.Context) layout.Dimensions {
	v := m.getReverseBaseWidth()
	gtx.Constraints.Min.X = int(m.getBaseWidth())
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
	v := m.getBaseWidth()
	gtx.Constraints.Min.X = int(v)
	macro := op.Record(gtx.Ops)
	d := gif.Layout(gtx, WidthFixed)
	call := macro.Stop()
	m.drawBorder(gtx, d, call)
	return d
}

func (m *Message) drawBrokenImage(gtx layout.Context) layout.Dimensions {
	m.imageBroken = true
	v := m.getReverseBaseWidth()
	gtx.Constraints.Min.X = int(v)
	macro := op.Record(gtx.Ops)
	d := icons.ImageBrokenIcon.Layout(gtx, m.Theme.ContrastFg)
	call := macro.Stop()
	m.drawBorder(gtx, d, call)
	return d
}

func (m *Message) drawImage(gtx layout.Context, img image.Image) layout.Dimensions {
	v := m.getBaseWidth()
	dx := img.Bounds().Dx()
	dy := img.Bounds().Dy()
	point := image.Point{X: int(v), Y: int(float32(dy) / float32(dx) * v)}
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
	bgColor.A = 128
	radius := gtx.Dp(16)
	sE, sW, nW, nE := radius, radius, radius, radius
	if m.isMe() {
		nE = 0
	} else {
		nW = 0
	}
	defer clip.RRect{Rect: image.Rectangle{
		Max: d.Size,
	}, SE: sE, SW: sW, NW: nW, NE: nE}.Push(gtx.Ops).Pop()
	m.addInteractiveSpan(gtx)
	component.Rect{Color: bgColor, Size: d.Size}.Layout(gtx)
	// draw text
	call.Add(gtx.Ops)
}

func (m *Message) addInteractiveSpan(gtx layout.Context) {
	if m.InteractiveSpan.pressing {
		defer pointer.StopOp{}.Push(gtx.Ops).Pop()
	}
	m.InteractiveSpan.Layout(gtx)
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
			icon = icons.AlertErrorIcon
			iconColor = color.NRGBA(colornames.Red500)
		case Sent:
			icon = icons.ActionDoneIcon
		case Read:
			icon = icons.ActionDoneAllIcon
		}
		return icon.Layout(gtx, iconColor)
	}
	return layout.Dimensions{}
}

func (m *Message) drawName(gtx layout.Context) layout.Dimensions {
	timeVal := m.CreatedAt
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		loc = ShanghaiLoc
	}
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
var InvalidateRequest = make(chan struct{}, 1)
var HintRequest = make(chan string, 1)
