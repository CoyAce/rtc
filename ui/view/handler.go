package view

import (
	"log"
	"os"
	"path/filepath"
	modal "rtc/ui/layout"
	"unsafe"

	"github.com/CoyAce/wi"
)

var OnSettingsSubmit = func() {
	modal.DefaultModal.Dismiss(nil)
}
var SyncIcon = func() {
	path := GetPath(wi.DefaultClient.FullID(), "icon.png")
	i, err := os.Stat(path)
	if err != nil {
		log.Printf("%v stat failed, %v", path, err)
		path = GetPath(wi.DefaultClient.FullID(), "icon.gif")
		i, err = os.Stat(path)
		if err != nil {
			log.Printf("%v stat failed, %v", path, err)
			return
		}
	}
	err = wi.DefaultClient.SendFile(Content(path), wi.OpSyncIcon, wi.Hash(unsafe.Pointer(&i)), filepath.Base(path), uint64(i.Size()), 0)
	if err != nil {
		log.Printf("SyncIcon failed, %v", err)
	}
}
