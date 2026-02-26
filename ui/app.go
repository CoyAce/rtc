package ui

import (
	"log"
	"rtc/internal/audio"
	ui "rtc/ui/layout"
	"rtc/ui/native"
	"rtc/ui/view"
	"runtime"

	"gioui.org/app"
	"gioui.org/io/event"
	"gioui.org/op"
	"github.com/CoyAce/wi"
	"github.com/gen2brain/malgo"
)

func Draw(window *app.Window, c *wi.Client) error {
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
	m := view.NewMessageManager(audio.NewStreamConfig(maCtx, 1))
	m.Process(window, c)
	// ops are the operations from the UI
	var ops op.Ops
	// listen for events in the window.
	for {
		evt := window.Event()
		listenEvents(evt)
		// detect what type of event
		switch e := evt.(type) {
		// this is sent when the application is closed
		case app.DestroyEvent:
			return e.Err
		case app.ConfigEvent:
			if e.Config.Focused == false {
				wi.DefaultClient.Store()
				m.MessageKeeper.Flush()
			}
			if runtime.GOOS == "android" || runtime.GOOS == "ios" {
				if e.Config.Focused == false {
					wi.DefaultClient.SignOut()
				}
			}
		// this is sent when the application should re-render.
		case app.FrameEvent:
			// This graphics context is used for managing the rendering state.
			gtx := app.NewContext(&ops, e)

			m.Layout(gtx)
			ui.DefaultModal.Layout(gtx)

			// Pass the drawing operations to the GPU.
			e.Frame(gtx.Ops)
		}
	}
}

func listenEvents(event event.Event) {
	view.Picker.ListenEvents(event)
	native.Tool.ListenEvents(event)
}
