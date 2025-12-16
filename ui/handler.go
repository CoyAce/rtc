package ui

import (
	"image"
	"log"
	"rtc/core"

	"gioui.org/layout"
)

var OnFileReceived = func(req core.WriteReq) {
	if req.Code == core.OpSyncIcon {
		if avatarCache[req.UUID] == nil {
			avatarCache[req.UUID] = &Avatar{UUID: req.UUID}
		}
		avatarCache[req.UUID].Reload()
	}
}
var OnSettingsSubmit = func(gtx layout.Context) {
	modal.Dismiss(nil)
}
var SyncCachedIcon = func() {
	err := client.SyncIcon(avatarCache[client.FullID()].Image)
	if err != nil {
		log.Printf("Failed to sync icon: %v", err)
	}
}

var SyncSelectedIcon = func(img image.Image) {
	err := client.SyncIcon(img)
	if err != nil {
		log.Printf("Failed to sync icon: %v", err)
	}
}
