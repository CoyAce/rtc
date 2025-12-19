package ui

import (
	"rtc/assets/fonts"
	"rtc/core"
	ui "rtc/ui/layout"
	"strings"
	"time"

	"gioui.org/app"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/widget"
	"gioui.org/x/component"
	"gioui.org/x/explorer"
)

func Draw(window *app.Window, c *core.Client) error {
	// save client to global pointer
	client = c
	// ops are the operations from the UI
	var ops op.Ops

	var messageList = &MessageList{List: layout.List{Axis: layout.Vertical}, Theme: theme}
	// listen for events in the messages channel
	go func() {
		for m := range client.SignedMessages {
			message := Message{Sent, theme, client.FullID(),
				string(m.Payload), m.Sign.UUID, time.Now()}
			message.AddTo(messageList)
			messageList.ScrollToEnd = true
			window.Invalidate()
		}
	}()
	// handle file received event
	client.HandleFileWith(OnFileReceived)
	// handle sync operation
	client.SyncFunc = SyncCachedIcon
	inputField := component.TextField{Editor: widget.Editor{Submit: true}}
	messageEditor := MessageEditor{InputField: &inputField, Theme: theme}
	iconStack := NewIconStack()
	picker = explorer.NewExplorer(window)
	// listen for events in the window.
	for {
		event := window.Event()
		picker.ListenEvents(event)
		// detect what type of event
		switch e := event.(type) {
		// this is sent when the application is closed
		case app.DestroyEvent:
			return e.Err

		// this is sent when the application should re-render.
		case app.FrameEvent:
			// This graphics context is used for managing the rendering state.
			gtx := app.NewContext(&ops, e)

			// ---------- Handle input ----------
			if messageEditor.Submitted(gtx) {
				msg := strings.TrimSpace(inputField.Text())
				message := Message{Stateless, theme, client.FullID(),
					msg, client.FullID(), time.Now()}
				if client.Connected && client.SendText(msg) == nil {
					message.State = Sent
				}
				messageList.ScrollToEnd = true
				message.AddTo(messageList)
				inputField.Clear()
			}

			layout.Flex{Axis: layout.Vertical, Spacing: layout.SpaceBetween}.Layout(gtx,
				layout.Flexed(1, messageList.Layout),
				layout.Rigid(messageEditor.Layout),
			)
			iconStack.Layout(gtx)
			modal.Layout(gtx)

			// Pass the drawing operations to the GPU.
			e.Frame(gtx.Ops)
		}
	}
}

// theme defines the material design style
var client *core.Client
var theme = fonts.NewTheme()
var modal = ui.NewModalStack()
var picker *explorer.Explorer
