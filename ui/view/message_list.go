package view

import (
	"bufio"
	"encoding/json"
	"image"
	"log"
	"mushin/assets/fonts"
	"mushin/assets/icons"
	"mushin/internal/audio"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"sync/atomic"
	"time"

	"gioui.org/app"
	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"gioui.org/x/component"
	"github.com/CoyAce/wi"
	"github.com/gen2brain/malgo"
)

type VoiceMode bool

type MessageManager struct {
	*VoiceMode
	*MessageList
	*MessageKeeper
	*VoiceRecorder
	*MessageEditor
	*Hint
	audioStack *IconStack
	iconStack  *IconStack
}

func (v *VoiceMode) SwitchBetweenTextAndVoice(voiceMessage *IconButton) func() {
	return func() {
		*v = !*v
		if *v {
			voiceMessage.VGData = icons.CommunicationChatBubble
		} else {
			voiceMessage.VGData = icons.AVMic
		}
	}
}

func (m *MessageManager) Process(window *app.Window, c *wi.Client) {
	go wi.DefaultClient.Pull()
	go ConsumeAudioData(m.StreamConfig)
	go m.MessageKeeper.Loop()
	go func() {
		for {
			select {
			case <-InvalidateRequest:
				window.Invalidate()
			case msg := <-HintRequest:
				m.Hint.MSG = msg
				m.Start(time.Now(), component.Forward, 1000*time.Millisecond)
				window.Invalidate()
			}
		}
	}()
	// listen for events in the messages channel
	go func() {
		for {
			var message *Message
			select {
			case msg := <-MessageBox:
				if msg == nil {
					log.Printf("nil message")
					continue
				}
				message = msg
			case msg := <-c.SignedMessages:
				AvatarCache.LoadOrElseNew(msg.UUID).Load()
				message = &Message{
					State:       Sent,
					TextControl: NewTextControl(string(msg.Payload)),
					MessageStyle: MessageStyle{
						Theme: fonts.DefaultTheme,
					},
					Contacts:    FromSender(msg.UUID),
					MessageType: Text,
					CreatedAt:   time.UnixMilli(msg.CreatedAt),
					Sign:        wi.DefaultClient.Sign,
					Block:       msg.Block,
				}
			case msg := <-c.SubMessages:
				m.publishContent(msg)
				continue
			case msg := <-c.CtrlMessages:
				if msg.Code == wi.OpSyncName {
					go copyThenReloadIcon(msg.Target, msg.UUID)
				}
				continue
			case msg := <-c.FileMessages:
				message = &Message{
					State: Sent,
					MessageStyle: MessageStyle{
						Theme: fonts.DefaultTheme,
					},
					Contacts:    FromSender(msg.UUID),
					FileControl: FileControl{Filename: msg.Filename},
					CreatedAt:   time.UnixMilli(msg.CreatedAt),
					Sign:        wi.DefaultClient.Sign,
					Block:       msg.Block,
				}
				switch msg.Code {
				case wi.OpSendImage:
					message.MessageType = Image
				case wi.OpSendGif:
					message.MessageType = GIF
				case wi.OpSendVoice:
					mediaControl := MediaControl{StreamConfig: m.StreamConfig, Duration: msg.Duration}
					mediaControl.Format = malgo.FormatS16
					message.MessageType = Voice
					message.MediaControl = mediaControl
				case wi.OpPublish:
					fileControl := FileControl{
						Filename: msg.Filename,
						FileId:   msg.FileId,
						Size:     msg.Size,
						Mime:     NewMine(msg.Filename),
					}
					m.MessageKeeper.AppendDownloadable(&FileDescription{
						ID: msg.FileId, Name: msg.Filename, Size: int64(msg.Size),
					})
					message.MessageType = File
					message.FileControl = fileControl
				default:
					m.handleOp(msg)
					continue
				}
			}
			message.AddTo(m.MessageList)
			message.SendTo(m.MessageKeeper)
			m.MessageList.ScrollToEnd = true
			window.Invalidate()
		}
	}()
}
func (m *MessageManager) handleOp(req wi.WriteReq) {
	switch req.Code {
	case wi.OpSyncIcon:
		m.reloadAvatar(req)
	case wi.OpAudioCall:
		ShowIncomingCall(req)
	case wi.OpAcceptAudioCall:
		go PostAudioCallAccept(m.StreamConfig)
	case wi.OpEndAudioCall:
		EndIncomingCall()
	case wi.OpContent:
		if req.FileId == 0 {
			_ = wi.DefaultClient.UnsubscribeFile(0, req.UUID)
			// update icon
			m.reloadAvatar(req)
			return
		}
		fd := m.findDownloadableFile(req.FileId)
		if fd != nil {
			m.MessageKeeper.AppendDownloaded(fd)
			_ = wi.DefaultClient.UnsubscribeFile(req.FileId, req.UUID)
		}
	default:
	}
}

func (m *MessageManager) reloadAvatar(req wi.WriteReq) {
	avatar := AvatarCache.LoadOrElseNew(req.UUID)
	if filepath.Ext(req.Filename) == ".gif" {
		avatar.Reload(GIF_IMG)
	} else {
		avatar.Reload(IMG)
	}
}

func (m *MessageManager) publishContent(msg wi.ReadReq) {
	log.Printf("subscribe req received %v", msg)
	if msg.FileId == 0 {
		//send icon
		PublishIcon()
		return
	}
	fd := m.findPublishedFile(msg.FileId)
	if fd != nil {
		PublishContent(fd)
	}
}

func (m *MessageManager) findPublishedFile(id uint32) *FileDescription {
	m.MessageKeeper.lock.Lock()
	defer m.MessageKeeper.lock.Unlock()
	return m.MessageKeeper.PublishedFiles[id]
}

func (m *MessageManager) findDownloadableFile(id uint32) *FileDescription {
	m.MessageKeeper.lock.Lock()
	defer m.MessageKeeper.lock.Unlock()
	return m.MessageKeeper.DownloadableFiles[id]
}

func (m *MessageManager) Layout(gtx layout.Context) {
	// Draw dark geek-themed background
	defer clip.Rect{Max: gtx.Constraints.Max}.Push(gtx.Ops).Pop()
	paint.FillShape(gtx.Ops, m.MessageList.Bg, clip.Rect{Max: gtx.Constraints.Max}.Op())

	w := m.MessageEditor.Layout
	if *m.VoiceMode {
		w = m.VoiceRecorder.Layout
	}
	layout.Flex{Axis: layout.Vertical, Spacing: layout.SpaceBetween}.Layout(gtx,
		layout.Flexed(1, m.MessageList.Layout),
		layout.Rigid(layout.Spacer{Height: unit.Dp(7)}.Layout),
		layout.Rigid(w),
	)
	m.Hint.Layout(gtx)
	_, d := m.audioStack.Layout(gtx)
	op.Offset(image.Pt(0, -d.Size.Y)).Add(gtx.Ops)
	m.iconStack.Layout(gtx)
}

func NewMessageManager(streamConfig audio.StreamConfig) MessageManager {
	mode := new(VoiceMode)
	voiceRecorder := &VoiceRecorder{StreamConfig: streamConfig}
	messageKeeper := &MessageKeeper{
		MessageChannel: make(chan *Message, 1),
	}
	messageList := &MessageList{
		List:  layout.List{Axis: layout.Vertical, ScrollToEnd: true, Gap: int(unit.Dp(24))},
		Theme: fonts.DefaultTheme,
	}
	messageList.Messages.Store(new(messageKeeper.Messages(streamConfig)))
	messageEditor := &MessageEditor{Editor: widget.Editor{Submit: true, LineHeight: fonts.DefaultLineHeight}, Theme: fonts.DefaultTheme}
	return MessageManager{
		audioStack:    NewAudioIconStack(streamConfig),
		iconStack:     NewIconStack(mode.SwitchBetweenTextAndVoice, messageKeeper.AppendPublish),
		VoiceMode:     mode,
		Hint:          &Hint{MSG: "✅完成", Progress: &component.Progress{}},
		VoiceRecorder: voiceRecorder,
		MessageList:   messageList,
		MessageKeeper: messageKeeper,
		MessageEditor: messageEditor,
	}
}

type MessageList struct {
	layout.List
	*material.Theme
	widget.Clickable
	Messages atomic.Pointer[[]*Message]
}

func (l *MessageList) Layout(gtx layout.Context) layout.Dimensions {
	l.getFocusAndResetIconStackIfClicked(gtx)
	// We visualize the text using a list where each paragraph is a separate item.
	messages := *l.Messages.Load()
	dimensions := l.Clickable.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return l.List.Layout(gtx, len(messages), func(gtx layout.Context, index int) layout.Dimensions {
			return messages[index].Layout(gtx)
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
		messages := *l.Messages.Load()
		if l.Position.First+l.Position.Count < len(messages) {
			l.ScrollToEnd = true
		}
	}
}

func (l *MessageList) getFocusAndResetIconStackIfClicked(gtx layout.Context) {
	if l.Clicked(gtx) {
		gtx.Execute(key.FocusCmd{Tag: &l.Clickable})
		gtx.Execute(op.InvalidateCmd{})
	}
}

type MessageKeeper struct {
	MessageChannel    chan *Message
	buffer            []*Message
	DownloadableFiles map[uint32]*FileDescription
	PublishedFiles    map[uint32]*FileDescription
	DownloadedFiles   map[uint32]*FileDescription
	lock              sync.Mutex
}

func (k *MessageKeeper) Loop() {
	const flushFreq = 1 * time.Minute
	timer := time.NewTimer(flushFreq)
	for {
		select {
		case msg := <-k.MessageChannel:
			k.lock.Lock()
			k.buffer = append(k.buffer, msg)
			k.lock.Unlock()
		case <-timer.C:
			timer.Reset(flushFreq)
			if len(k.buffer) == 0 {
				continue
			}
			k.Flush()
		}
	}
}

func (k *MessageKeeper) Flush() {
	filePath := GetDataPath("message.log")
	file, err := os.OpenFile(filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Printf("Open file failed: %v", err)
	}
	defer file.Close()
	k.lock.Lock()
	defer k.lock.Unlock()
	for _, msg := range k.buffer {
		k.writeJson(file, msg)
	}
	k.buffer = k.buffer[:0]
}

func (k *MessageKeeper) AppendPublish(fd *FileDescription) {
	k.lock.Lock()
	defer k.lock.Unlock()
	k.PublishedFiles[fd.ID] = fd
	k.append("file.log", fd)
}

func (k *MessageKeeper) AppendDownloaded(fd *FileDescription) {
	k.lock.Lock()
	defer k.lock.Unlock()
	k.DownloadedFiles[fd.ID] = fd
	k.append("download.log", fd)
}

func (k *MessageKeeper) AppendDownloadable(fd *FileDescription) {
	k.lock.Lock()
	defer k.lock.Unlock()
	k.DownloadableFiles[fd.ID] = fd
	k.append("downloadable.log", fd)
}

func (k *MessageKeeper) append(filename string, fd *FileDescription) {
	filePath := GetDataPath(filename)
	file, err := os.OpenFile(filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Printf("Open file failed: %v", err)
	}
	defer file.Close()
	k.writeJson(file, fd)
}

func (k *MessageKeeper) writeJson(file *os.File, msg any) {
	s, err := json.Marshal(msg)
	if err != nil {
		log.Printf("Marshall failed: %v", err)
		return
	}
	_, err = file.WriteString(string(s) + "\n")
	if err != nil {
		log.Printf("Write file failed: %v", err)
	}
}

func (k *MessageKeeper) ReadPublishedFiles() map[uint32]*FileDescription {
	return k.read("file.log")
}

func (k *MessageKeeper) ReadDownloadedFiles() map[uint32]*FileDescription {
	return k.read("download.log")
}

func (k *MessageKeeper) ReadDownloadableFiles() map[uint32]*FileDescription {
	return k.read("downloadable.log")
}

func (k *MessageKeeper) read(filename string) map[uint32]*FileDescription {
	ret := make(map[uint32]*FileDescription)
	filePath := GetDataPath(filename)
	_, err := os.Stat(filePath)
	if err != nil {
		return ret
	}
	f, err := os.Open(filePath)
	if err != nil {
		log.Printf("Open file failed: %v", err)
		return ret
	}
	s := bufio.NewScanner(f)
	for s.Scan() {
		var fd FileDescription
		line := s.Bytes()
		err = json.Unmarshal(line, &fd)
		if err != nil {
			log.Printf("Unmarshall file mapping failed: %v", err)
		}
		ret[fd.ID] = &fd
	}
	return ret
}

func (k *MessageKeeper) Messages(streamConfig audio.StreamConfig) []*Message {
	k.PublishedFiles = k.ReadPublishedFiles()
	k.DownloadedFiles = k.ReadDownloadedFiles()
	k.DownloadableFiles = k.ReadDownloadableFiles()
	filePath := GetDataPath("message.log")
	f, err := os.Open(filePath)
	if err != nil {
		log.Printf("Open file failed: %v", err)
		return []*Message{}
	}
	ret := make([]*Message, 0, 32)
	s := bufio.NewScanner(f)
	for s.Scan() {
		var msg Message
		line := s.Bytes()
		err = json.Unmarshal(line, &msg)
		if err != nil {
			log.Printf("Unmarshall message failed: %v", err)
		}
		msg.TextControl = NewTextControl(msg.Text)
		msg.Theme = fonts.DefaultTheme
		if msg.State == Stateless {
			msg.State = Failed
		}
		if msg.MessageType == Voice {
			msg.StreamConfig = streamConfig
		}
		if k.DownloadedFiles[msg.FileId] != nil {
			msg.progress = 100
		}
		if !msg.isMe() {
			wi.DefaultClient.Track(&wi.SignBody{Sign: msg.Sign, UUID: msg.Sender}, msg.Block)
		} else {
			wi.DefaultClient.MultiTrack(&wi.SignBody{Sign: msg.Sign, UUID: msg.Sender}, wi.FullRange)
		}
		ret = append(ret, &msg)
	}
	slices.SortFunc(ret, func(i, j *Message) int {
		switch {
		case i.CreatedAt.Before(j.CreatedAt):
			return -1
		case i.CreatedAt.After(j.CreatedAt):
			return 1
		default:
			return 0
		}
	})
	adjustPrimaryForAll(ret)
	return ret
}

func adjustPrimaryForAll(ret []*Message) {
	for i := len(ret) - 1; i >= 0; i-- {
		if i == len(ret)-1 {
			ret[i].Primary = ret[i].isMe()
			ret[i].nextSame = false
			continue
		}
		adjustPrimary(ret, i)
	}
}

func adjustPrimary(ret []*Message, i int) {
	if ret[i].Sender == ret[i+1].Sender || ret[i].isMe() && ret[i+1].isMe() {
		ret[i].Primary = ret[i+1].Primary
		ret[i].nextSame = true
	} else {
		ret[i].Primary = !ret[i+1].Primary
		ret[i].nextSame = false
	}
}

type Hint struct {
	*component.Progress
	MSG string
}

func (h *Hint) Layout(gtx layout.Context) layout.Dimensions {
	if !h.Started() {
		return layout.Dimensions{}
	}
	gtx.Execute(op.InvalidateCmd{})
	h.Progress.Update(gtx.Now)
	return layout.Stack{Alignment: layout.S}.Layout(gtx,
		layout.Stacked(func(gtx layout.Context) layout.Dimensions {
			offset := image.Pt(0, -gtx.Dp(57+4))
			op.Offset(offset).Add(gtx.Ops)
			macro := op.Record(gtx.Ops)
			margins := layout.Inset{Top: unit.Dp(2), Bottom: unit.Dp(3), Left: unit.Dp(4), Right: unit.Dp(4)}
			d := margins.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				label := material.Label(fonts.DefaultTheme, fonts.DefaultTheme.TextSize, h.MSG)
				label.Color = fonts.DefaultTheme.ContrastFg
				label.Alignment = text.Middle
				return label.Layout(gtx)
			})
			call := macro.Stop()
			bgColor := fonts.DefaultTheme.ContrastBg
			bgColor.A = 160
			component.Rect{Color: bgColor, Size: d.Size, Radii: gtx.Dp(4)}.Layout(gtx)
			call.Add(gtx.Ops)
			return d
		}),
	)
}
