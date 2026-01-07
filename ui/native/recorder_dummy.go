//go:build !android

package native

import "gioui.org/io/event"

type Recorder struct {
}

func (r *Recorder) ListenEvents(evt event.Event) {
}

func (r *Recorder) AskPermission() {
}
