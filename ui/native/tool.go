package native

import (
	"gioui.org/app"
)

var Tool *PlatformTool

func NewPlatformTool(w *app.Window) (r *PlatformTool) {
	r = &PlatformTool{
		window: w,
	}
	return r
}
