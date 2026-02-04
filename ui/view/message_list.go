package view

import (
	"bufio"
	"encoding/json"
	"log"
	"os"
	"rtc/assets/fonts"
	"rtc/internal/audio"
	"sync"
	"time"

	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/widget"
	"gioui.org/widget/material"
)

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
		iconStackAnimation.Disappear(gtx.Now)
	}
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
	filePath := GetDataPath("message.log")
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
	filePath := GetDataPath("message.log")
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
		if msg.MessageType == Voice {
			msg.StreamConfig = k.StreamConfig
		}
		ret = append(ret, &msg)
	}
	return ret
}
