package view

import (
	"bufio"
	"encoding/json"
	"image"
	"log"
	"os"
	"path/filepath"
	"rtc/assets/fonts"
	"rtc/internal/audio"
	"sync"
	"time"

	"gioui.org/app"
	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"gioui.org/x/component"
	"github.com/CoyAce/wi"
)

type VoiceMode bool

type MessageManager struct {
	*VoiceMode
	*MessageList
	*MessageKeeper
	*VoiceRecorder
	*MessageEditor
	audioStack *IconStack
	iconStack  *IconStack
}

func (v *VoiceMode) SwitchBetweenTextAndVoice(voiceMessage *IconButton) func() {
	return func() {
		*v = !*v
		if *v {
			voiceMessage.Icon = chatIcon
		} else {
			voiceMessage.Icon = voiceMessageIcon
		}
	}
}

func (m *MessageManager) Process(window *app.Window, c *wi.Client) {
	go ConsumeAudioData(m.StreamConfig)
	go m.MessageKeeper.Loop()
	// listen for events in the messages channel
	go func() {
		handleOp := func(req wi.WriteReq) {
			switch req.Code {
			case wi.OpSyncIcon:
				avatar := AvatarCache.LoadOrElseNew(req.UUID)
				if filepath.Ext(req.Filename) == ".gif" {
					avatar.Reload(GIF_IMG)
				} else {
					avatar.Reload(IMG)
				}
			case wi.OpAudioCall:
				ShowIncomingCall(req)
			case wi.OpAcceptAudioCall:
				go PostAudioCallAccept(m.StreamConfig)
			case wi.OpEndAudioCall:
				EndIncomingCall(req.FileId == 0)
			default:
			}
		}
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
				text := string(msg.Payload)
				message = &Message{
					State:       Sent,
					TextControl: NewTextControl(text),
					Theme:       fonts.DefaultTheme,
					Contacts:    FromSender(msg.Sign.UUID),
					MessageType: Text,
					CreatedAt:   time.Now(),
				}
			case msg := <-c.FileMessages:
				message = &Message{
					State:       Sent,
					Theme:       fonts.DefaultTheme,
					Contacts:    FromSender(msg.UUID),
					FileControl: FileControl{Filename: msg.Filename},
					CreatedAt:   time.Now()}
				switch msg.Code {
				case wi.OpSendImage:
					message.MessageType = Image
				case wi.OpSendGif:
					message.MessageType = GIF
				case wi.OpSendVoice:
					mediaControl := MediaControl{StreamConfig: m.StreamConfig, Duration: msg.Duration}
					message.MessageType = Voice
					message.MediaControl = mediaControl
				case wi.OpPublish:
					fileControl := FileControl{
						Filename: msg.Filename,
						FileId:   msg.FileId,
						Size:     msg.Size,
						Mime:     NewMine(msg.Filename),
					}
					message.MessageType = File
					message.FileControl = fileControl
				default:
					handleOp(msg)
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

func (m *MessageManager) Layout(gtx layout.Context) {
	w := m.MessageEditor.Layout
	if *m.VoiceMode {
		w = m.VoiceRecorder.Layout
	}
	layout.Flex{Axis: layout.Vertical, Spacing: layout.SpaceBetween}.Layout(gtx,
		layout.Flexed(1, m.MessageList.Layout),
		layout.Rigid(w),
	)
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
		List:     layout.List{Axis: layout.Vertical, ScrollToEnd: true},
		Theme:    fonts.DefaultTheme,
		Messages: messageKeeper.Messages(streamConfig),
	}
	inputField := component.TextField{Editor: widget.Editor{Submit: true}}
	messageEditor := &MessageEditor{InputField: &inputField, Theme: fonts.DefaultTheme}
	return MessageManager{
		audioStack:    NewAudioIconStack(streamConfig),
		iconStack:     NewIconStack(mode.SwitchBetweenTextAndVoice, messageKeeper.Append),
		VoiceMode:     mode,
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
	Messages []*Message
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
		gtx.Execute(op.InvalidateCmd{})
	}
}

type FileMapping struct {
	ID   uint32
	Path string
}

type MessageKeeper struct {
	MessageChannel chan *Message
	buffer         []*Message
	lock           sync.Mutex
}

func (k *MessageKeeper) Loop() {
	flushFreq := 1 * time.Second
	timer := time.NewTimer(flushFreq)
	for {
		select {
		case msg := <-k.MessageChannel:
			k.lock.Lock()
			k.buffer = append(k.buffer, msg)
			k.lock.Unlock()
		case <-timer.C:
			if len(k.buffer) == 0 {
				continue
			}
			k.Flush()
			timer.Reset(flushFreq)
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

func (k *MessageKeeper) Append(fm *FileMapping) {
	filePath := GetDataPath("file.log")
	file, err := os.OpenFile(filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Printf("Open file failed: %v", err)
	}
	defer file.Close()
	k.writeJson(file, fm)
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

func (k *FileMapping) Mappings() []*FileMapping {
	filePath := GetDataPath("file.log")
	f, err := os.Open(filePath)
	if err != nil {
		log.Printf("Open file failed: %v", err)
		return []*FileMapping{}
	}
	ret := make([]*FileMapping, 0, 32)
	s := bufio.NewScanner(f)
	for s.Scan() {
		var fm FileMapping
		line := s.Bytes()
		err = json.Unmarshal(line, &fm)
		if err != nil {
			log.Printf("Unmarshall file mapping failed: %v", err)
		}
		ret = append(ret, &fm)
	}
	return ret
}

func (k *MessageKeeper) Messages(streamConfig audio.StreamConfig) []*Message {
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
		ret = append(ret, &msg)
	}
	return ret
}
