package view

import (
	"log"
	"rtc/assets/fonts"
	"time"
	"unsafe"

	"github.com/CoyAce/wi"
)

func ChooseAndSendFile(appendFile func(*FileMapping)) func() {
	return func() {
		go func() {
			fd, err := ChooseFile()
			if err != nil {
				log.Printf("Choose file failed: %v", err)
				return
			}
			id := wi.Hash(unsafe.Pointer(&fd))
			fc := FileControl{
				Filename: fd.Name,
				FileId:   id,
				Path:     fd.Path,
				Size:     uint64(fd.Size),
				Mime:     NewMine(fd.Name),
			}
			message := &Message{
				State:       Stateless,
				Theme:       fonts.DefaultTheme,
				FileControl: fc,
				Contacts:    FromMyself(),
				MessageType: File,
				CreatedAt:   time.Now(),
			}
			MessageBox <- message
			appendFile(&FileMapping{ID: id, Path: fd.Path})
			err = wi.DefaultClient.PublishFile(fd.Name, uint64(fd.Size), id)
			if err != nil {
				log.Printf("Publish file failed, %v", err)
				message.State = Failed
			} else {
				message.State = Sent
			}
		}()
	}
}
