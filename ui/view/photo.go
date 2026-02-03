package view

import (
	"log"
	"path/filepath"
	"rtc/assets/fonts"
	"time"

	"gioui.org/layout"
	"github.com/CoyAce/whily"
)

func ChooseAndSendPhoto(gtx layout.Context) {
	iconStackAnimation.Disappear(gtx.Now)
	go func() {
		fd, err := ChooseImage()
		if err != nil {
			log.Printf("choose image failed, %v", err)
			return
		}
		defer fd.File.Close()
		mType := Image
		opCode := whily.OpSendImage
		isGif := filepath.Ext(fd.Name) == ".gif"
		if isGif {
			mType = GIF
			opCode = whily.OpSendGif
		}
		message := &Message{State: Stateless, Theme: fonts.DefaultTheme, Contacts: FromMyself(),
			MessageType: mType, FileControl: FileControl{Path: fd.Path}, CreatedAt: time.Now()}
		MessageBox <- message
		err = whily.DefaultClient.SendFile(fd.File, opCode, fd.Name, uint64(fd.Size), 0)
		if err != nil {
			log.Printf("send image failed, %v", err)
			message.State = Failed
		} else {
			message.State = Sent
		}
	}()
}
