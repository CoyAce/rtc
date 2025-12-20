package ui

import (
	"rtc/core"
	"testing"
)

func TestMessagePersistence(t *testing.T) {
	client = &core.Client{UUID: "#00001"}
	core.Mkdir(core.GetDir(client.FullID()))
	core.RemoveFile(core.GetFileName(client.FullID(), "message.log"))
	mk := MessageKeeper{MessageChannel: make(chan *Message, 1)}
	go mk.Loop()
	mk.MessageChannel <- &Message{Text: "hello world", Sender: "test#00001", UUID: "#00001"}
	mk.MessageChannel <- &Message{Text: "hello beautiful world", Sender: "test#00001", UUID: "#00001"}
	mk.Append()
	messages := mk.Messages()
	if len(messages) != 2 {
		t.Errorf("Messages length should be 2, but %d", len(messages))
	}
	if messages[0].UUID != "#00001" {
		t.Errorf("Messages[0].UUID should be #00001, but %s", messages[0].UUID)
	}
	if messages[1].Text != "hello beautiful world" {
		t.Errorf("Messages[1].Text != hello beautiful world, but %s", messages[1].Text)
	}
}
