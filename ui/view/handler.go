package view

import (
	"image"
	"log"
	"rtc/core"

	modal "rtc/ui/layout"

	"gioui.org/layout"
)

var OnFileReceived = func(req core.WriteReq) {
	if req.Code == core.OpSyncIcon {
		if AvatarCache[req.UUID] == nil {
			AvatarCache[req.UUID] = &Avatar{UUID: req.UUID}
		}
		AvatarCache[req.UUID].Reload()
	}
}
var OnSettingsSubmit = func(gtx layout.Context) {
	modal.DefaultModal.Dismiss(nil)
}
var SyncCachedIcon = func() {
	avatar := AvatarCache[core.DefaultClient.FullID()]
	if avatar == nil || avatar.Image == nil {
		log.Printf("avatar not found in cache")
		return
	}
	go func() {
		err := core.DefaultClient.SyncIcon(avatar.Image)
		if err != nil {
			log.Printf("Failed to sync icon: %v", err)
		}
	}()
}

var SyncSelectedIcon = func(img image.Image) {
	go func() {
		err := core.DefaultClient.SyncIcon(img)
		if err != nil {
			log.Printf("Failed to sync icon: %v", err)
		}
	}()
}
