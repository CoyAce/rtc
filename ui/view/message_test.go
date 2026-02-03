package view

import (
	"testing"

	"github.com/CoyAce/whily"
)

func TestMessagePersistence(t *testing.T) {
	whily.DefaultClient = &whily.Client{UUID: "#00001"}
	whily.Mkdir(GetDir(whily.DefaultClient.FullID()))
	whily.RemoveFile(GetDataPath("message.log"))
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
