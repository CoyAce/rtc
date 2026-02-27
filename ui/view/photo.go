package view

import (
	"log"
	"path/filepath"
	"rtc/assets/fonts"
	"time"
	"unsafe"

	"github.com/CoyAce/wi"
)

func ChooseAndSendPhoto() {
	go func() {
		fd, err := ChooseImage()
		if err != nil {
			log.Printf("choose image failed, %v", err)
			return
		}
		mType := Image
		opCode := wi.OpSendImage
		isGif := filepath.Ext(fd.Name) == ".gif"
		if isGif {
			mType = GIF
			opCode = wi.OpSendGif
		}
		message := &Message{
			State: Stateless,
			MessageStyle: MessageStyle{
				Theme: fonts.DefaultTheme,
			},
			Contacts:    FromMyself(),
			MessageType: mType,
			FileControl: FileControl{Path: fd.Path, Filename: fd.Name},
			CreatedAt:   time.Now(),
		}
		MessageBox <- message
		err = wi.DefaultClient.SendFile(fd.File, opCode, wi.Hash(unsafe.Pointer(&fd)), fd.Name, uint64(fd.Size), 0)
		if err != nil {
			log.Printf("send image failed, %v", err)
			message.State = Failed
		} else {
			message.State = Sent
		}
	}()
}
