package view

import (
	"image"
	"image/gif"
	"log"

	modal "rtc/ui/layout"

	"gioui.org/layout"
	"github.com/CoyAce/whily"
)

var OnSettingsSubmit = func(gtx layout.Context) {
	modal.DefaultModal.Dismiss(nil)
}
var SyncCachedIcon = func() {
	avatar := AvatarCache.LoadOrElseNew(whily.DefaultClient.FullID())
	switch avatar.AvatarType {
	case Default:
		if avatar.Gif != nil {
			SyncSelectedIcon(nil, avatar.GIF)
		}
		fallthrough
	case IMG:
		if avatar.Image == nil || *avatar.Image == nil {
			return
		}
		SyncSelectedIcon(*avatar.Image, nil)
	case GIF_IMG:
		if avatar.Gif != nil {
			SyncSelectedIcon(nil, avatar.GIF)
		}
	}
}

var SyncSelectedIcon = func(img image.Image, gifImg *gif.GIF) {
	go func() {
		if img == nil {
			err := whily.DefaultClient.SyncGif(gifImg)
			if err != nil {
				log.Printf("Failed to sync icon: %v", err)
			}
			return
		}
		err := whily.DefaultClient.SyncIcon(img)
		if err != nil {
			log.Printf("Failed to sync icon: %v", err)
		}
	}()
}
