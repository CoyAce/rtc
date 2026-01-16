package view

import (
	"image"
	"image/gif"
	"log"
	"rtc/core"

	modal "rtc/ui/layout"

	"gioui.org/layout"
)

var OnSettingsSubmit = func(gtx layout.Context) {
	modal.DefaultModal.Dismiss(nil)
}
var SyncCachedIcon = func() {
	avatar := AvatarCache.LoadOrElseNew(core.DefaultClient.FullID())
	if avatar.AvatarType == Default || avatar.GIF == nil {
		log.Printf("avatar not found in cache")
		return
	}
	if avatar.AvatarType == IMG {
		SyncSelectedIcon(avatar.Image, nil)
	} else {
		SyncSelectedIcon(nil, avatar.GIF)
	}
}

var SyncSelectedIcon = func(img image.Image, gifImg *gif.GIF) {
	go func() {
		if img == nil {
			err := core.DefaultClient.SyncGif(gifImg)
			if err != nil {
				log.Printf("Failed to sync icon: %v", err)
			}
			return
		}
		err := core.DefaultClient.SyncIcon(img)
		if err != nil {
			log.Printf("Failed to sync icon: %v", err)
		}
	}()
}
