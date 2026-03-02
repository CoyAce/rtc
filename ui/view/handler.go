package view

import (
	"log"
	modal "mushin/ui/layout"
	"os"
	"path/filepath"
	"unsafe"

	"github.com/CoyAce/wi"
)

var OnSettingsSubmit = func() {
	modal.DefaultModal.Dismiss(nil)
}
var SyncIcon = func() {
	paths := []string{
		GetPath(wi.DefaultClient.ID(), "icon.png"), GetPath(wi.DefaultClient.ID(), "icon.gif"),
	}
	for _, path := range paths {
		i, err := os.Stat(path)
		if err != nil {
			continue
		}
		err = wi.DefaultClient.SendFile(Content(path), wi.OpSyncIcon, wi.Hash(unsafe.Pointer(&i)), filepath.Base(path), uint64(i.Size()), 0)
		if err != nil {
			log.Printf("SyncIcon failed, %v", err)
		}
	}
}

var PublishIcon = func() {
	paths := []string{
		GetPath(wi.DefaultClient.ID(), "icon.png"), GetPath(wi.DefaultClient.ID(), "icon.gif"),
	}
	for _, path := range paths {
		i, err := os.Stat(path)
		if err != nil {
			continue
		}
		err = wi.DefaultClient.PublishContent(Content(path), filepath.Base(path), uint64(i.Size()), 0)
		if err != nil {
			log.Printf("Publish icon failed, %v", err)
		}
	}
}
