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
	avatar := avatarCache[client.FullID()]
	if avatar == nil || avatar.Image == nil {
		log.Printf("avatar not found in cache")
		return
	}
	go func() {
		err := client.SyncIcon(avatar.Image)
		if err != nil {
			log.Printf("Failed to sync icon: %v", err)
		}
	}()
}

var SyncSelectedIcon = func(img image.Image) {
	go func() {
		err := client.SyncIcon(img)
		if err != nil {
			log.Printf("Failed to sync icon: %v", err)
		}
	}()
}
