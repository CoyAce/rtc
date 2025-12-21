package ui

import (
	"rtc/assets/fonts"
	"rtc/core"
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
	"gioui.org/widget"
	"gioui.org/x/explorer"
)

func Draw(window *app.Window, c *core.Client) error {
	// save client to global pointer
	core.DefaultClient = c
	// ops are the operations from the UI
	var ops op.Ops

	var messageList = &view.MessageList{List: layout.List{Axis: layout.Vertical},
		Theme: fonts.DefaultTheme, ScrollToEnd: true}
	var messageKeeper = &view.MessageKeeper{MessageChannel: make(chan *view.Message, 1)}
	messageList.Messages = messageKeeper.Messages()
	go messageKeeper.Loop()
	// listen for events in the messages channel
	go func() {
		for {
			var message *view.Message
			select {
			case m := <-view.MessageBox:
				message = m
			case m := <-c.SignedMessages:
				text := string(m.Payload)
				ed := widget.Editor{ReadOnly: true}
				ed.SetText(text)
				message = &view.Message{State: view.Sent, Editor: &ed, Theme: fonts.DefaultTheme,
					UUID: core.DefaultClient.FullID(), Type: view.Text,
					Text: text, Sender: m.Sign.UUID, CreatedAt: time.Now()}
			case m := <-c.FileMessages:
				if m.Code == core.OpSyncIcon {
					if view.AvatarCache[m.UUID] == nil {
						view.AvatarCache[m.UUID] = &view.Avatar{UUID: m.UUID}
					}
					view.AvatarCache[m.UUID].Reload()
					continue
				}
				if m.Code == core.OpSendImage {
					message = &view.Message{State: view.Sent, Theme: fonts.DefaultTheme,
						UUID: core.DefaultClient.FullID(), Type: view.Image, Filename: m.Filename,
						Sender: m.UUID, CreatedAt: time.Now()}
				}
			}
			message.AddTo(messageList)
			message.SendTo(messageKeeper)
			messageList.ScrollToEnd = true
			window.Invalidate()
		}
	}()
	// handle sync operation
	core.DefaultClient.SyncFunc = view.SyncCachedIcon
	inputField := component.TextField{Editor: ui.Editor{Submit: true}}
	messageEditor := view.MessageEditor{InputField: &inputField, Theme: fonts.DefaultTheme}
	iconStack := view.NewIconStack()
	view.DefaultPicker = explorer.NewExplorer(window)
	if runtime.GOOS == "android" {
		view.DefaultPicker = native.NewExplorer(window)
	}
	// listen for events in the window.
	for {
		event := window.Event()
		view.DefaultPicker.ListenEvents(event)
		// detect what type of event
		switch e := event.(type) {
		// this is sent when the application is closed
		case app.DestroyEvent:
			core.DefaultClient.Store()
			messageKeeper.Append()
			return e.Err

		// this is sent when the application should re-render.
		case app.FrameEvent:
			// This graphics context is used for managing the rendering state.
			gtx := app.NewContext(&ops, e)

			// ---------- Handle input ----------
			if messageEditor.Submitted(gtx) {
				msg := strings.TrimSpace(inputField.Text())
				inputField.Clear()
				go func() {
					ed := widget.Editor{ReadOnly: true}
					ed.SetText(msg)
					message := view.Message{State: view.Stateless, Editor: &ed,
						Theme: fonts.DefaultTheme,
						UUID:  core.DefaultClient.FullID(), Type: view.Text,
						Text: msg, Sender: core.DefaultClient.FullID(), CreatedAt: time.Now()}
					view.MessageBox <- &message
					if core.DefaultClient.Connected && core.DefaultClient.SendText(msg) == nil {
						message.State = view.Sent
					} else {
						message.State = view.Failed
					}
				}()
			}

			layout.Flex{Axis: layout.Vertical, Spacing: layout.SpaceBetween}.Layout(gtx,
				layout.Flexed(1, messageList.Layout),
				layout.Rigid(messageEditor.Layout),
			)
			iconStack.Layout(gtx)
			ui.DefaultModal.Layout(gtx)

			// Pass the drawing operations to the GPU.
			e.Frame(gtx.Ops)
		}
	}
}
