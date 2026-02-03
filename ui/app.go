package ui

import (
	"image"
	"log"
	"path/filepath"
	"rtc/assets/fonts"
	"rtc/internal/audio"
	ui "rtc/ui/layout"
	"rtc/ui/layout/component"
	"rtc/ui/native"
	"rtc/ui/view"
	"runtime"
	"strings"
	"time"

	"gioui.org/app"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/x/explorer"
	"github.com/CoyAce/whily"
	"github.com/gen2brain/malgo"
)

func Draw(window *app.Window, c *whily.Client) error {
	// save client to global pointer
	whily.DefaultClient = c
	// audio
	maCtx, err := malgo.InitContext(nil, malgo.ContextConfig{}, func(message string) {
		log.Print("internal/audio: ", message)
	})
	if err != nil {
		log.Fatalln("main: ", err)
	}
	defer func() {
		_ = maCtx.Uninit()
		maCtx.Free()
	}()
	streamConfig := audio.NewStreamConfig(maCtx, 1)
	voiceRecorder := view.VoiceRecorder{StreamConfig: streamConfig}
	// ops are the operations from the UI
	var ops op.Ops

	var messageList = &view.MessageList{List: ui.List{Axis: ui.Vertical, ScrollToEnd: true},
		Theme: fonts.DefaultTheme}
	var messageKeeper = &view.MessageKeeper{MessageChannel: make(chan *view.Message, 1), StreamConfig: streamConfig}
	messageList.Messages = messageKeeper.Messages()
	go messageKeeper.Loop()
	go view.ConsumeAudioData(streamConfig)
	// listen for events in the messages channel
	go func() {
		handleOp := func(m whily.WriteReq) {
			switch m.Code {
			case whily.OpSyncIcon:
				avatar := view.AvatarCache.LoadOrElseNew(m.UUID)
				if filepath.Ext(m.Filename) == ".gif" {
					avatar.Reload(view.GIF_IMG)
				} else {
					avatar.Reload(view.IMG)
				}
			case whily.OpAudioCall:
				view.ShowIncomingCall(m)
			case whily.OpAcceptAudioCall:
				go view.PostAudioCallAccept(streamConfig)
			case whily.OpEndAudioCall:
				view.EndIncomingCall(m.FileId == 0)
			default:
			}
		}
		for {
			var message *view.Message
			select {
			case m := <-view.MessageBox:
				if m == nil {
					log.Printf("nil message")
					continue
				}
				message = m
			case m := <-c.SignedMessages:
				text := string(m.Payload)
				message = &view.Message{
					State:       view.Sent,
					TextControl: view.NewTextControl(text),
					Theme:       fonts.DefaultTheme,
					Contacts:    view.FromSender(m.Sign.UUID),
					MessageType: view.Text,
					CreatedAt:   time.Now(),
				}
			case m := <-c.FileMessages:
				message = &view.Message{
					State:       view.Sent,
					Theme:       fonts.DefaultTheme,
					Contacts:    view.FromSender(m.UUID),
					FileControl: view.FileControl{Filename: m.Filename},
					CreatedAt:   time.Now()}
				switch m.Code {
				case whily.OpSendImage:
					message.MessageType = view.Image
				case whily.OpSendGif:
					message.MessageType = view.GIF
				case whily.OpSendVoice:
					mediaControl := view.MediaControl{StreamConfig: streamConfig, Duration: m.Duration}
					message.MessageType = view.Voice
					message.MediaControl = mediaControl
				default:
					handleOp(m)
					continue
				}
			}
			message.AddTo(messageList)
			message.SendTo(messageKeeper)
			messageList.ScrollToEnd = true
			window.Invalidate()
		}
	}()
	// handle sync operation
	whily.DefaultClient.SyncFunc = view.SyncCachedIcon
	inputField := component.TextField{Editor: ui.Editor{Submit: true}}
	messageEditor := view.MessageEditor{InputField: &inputField, Theme: fonts.DefaultTheme}
	iconStack := view.NewIconStack()
	audioStack := view.NewAudioIconStack(streamConfig)
	view.DefaultPicker = explorer.NewExplorer(window)
	if runtime.GOOS == "android" {
		view.DefaultPicker = native.NewExplorer(window)
	}
	native.DefaultRecorder = native.NewRecorder(window)
	// listen for events in the window.
	for {
		event := window.Event()
		view.DefaultPicker.ListenEvents(event)
		native.DefaultRecorder.ListenEvents(event)
		// detect what type of event
		switch e := event.(type) {
		// this is sent when the application is closed
		case app.DestroyEvent:
			return e.Err
		case app.ConfigEvent:
			if e.Config.Focused == false {
				whily.DefaultClient.Store()
				messageKeeper.Append()
				view.ICache.Reset()
				view.GCache.Reset()
			}
		// this is sent when the application should re-render.
		case app.FrameEvent:
			// This graphics context is used for managing the rendering state.
			gtx := app.NewContext(&ops, e)

			// ---------- Handle input ----------
			if messageEditor.Submitted(gtx) {
				msg := strings.TrimSpace(inputField.Text())
				inputField.Clear()
				go func() {
					message := view.Message{State: view.Stateless,
						TextControl: view.NewTextControl(msg),
						Theme:       fonts.DefaultTheme,
						Contacts:    view.FromMyself(),
						MessageType: view.Text,
						CreatedAt:   time.Now()}
					view.MessageBox <- &message
					if whily.DefaultClient.Connected && whily.DefaultClient.SendText(msg) == nil {
						message.State = view.Sent
					} else {
						message.State = view.Failed
					}
				}()
			}
			w := messageEditor.Layout
			if view.VoiceMode {
				w = voiceRecorder.Layout
			}
			layout.Flex{Axis: layout.Vertical, Spacing: layout.SpaceBetween}.Layout(gtx,
				layout.Flexed(1, messageList.Layout),
				layout.Rigid(w),
			)
			_, d := audioStack.Layout(gtx)
			op.Offset(image.Pt(0, -d.Size.Y)).Add(gtx.Ops)
			iconStack.Layout(gtx)
			ui.DefaultModal.Layout(gtx)

			// Pass the drawing operations to the GPU.
			e.Frame(gtx.Ops)
		}
	}
}
