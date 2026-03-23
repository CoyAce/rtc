package icons

import (
	"gioui.org/widget"
	"golang.org/x/exp/shiny/materialdesign/icons"
)

//go:generate go run main.go

// IconVG data exports for glitch rendering
var (
	// Custom icons (already in data.go)
	// Apk, Audio, Browse, FileExport, Video
	
	// Material Design Icons - export raw VGData
	ActionDone           = icons.ActionDone
	ActionDoneAll        = icons.ActionDoneAll
	AlertErrorOutline    = icons.AlertErrorOutline
	ContentSend          = icons.ContentSend
	NavigationUnfoldMore = icons.NavigationUnfoldMore
	NavigationUnfoldLess = icons.NavigationUnfoldLess
	AVMic                = icons.AVMic
	CommunicationPhone   = icons.CommunicationPhone
	AVVideoCall          = icons.AVVideoCall
	AVMicOff             = icons.AVMicOff
	ActionSettings       = icons.ActionSettings
	ImagePhotoLibrary    = icons.ImagePhotoLibrary
	ImageBrokenImage     = icons.ImageBrokenImage
	NavigationRefresh    = icons.NavigationRefresh
	NotificationSync     = icons.NotificationSync
	ContentContentCut    = icons.ContentContentCut
	ContentContentCopy   = icons.ContentContentCopy
	ContentContentPaste  = icons.ContentContentPaste
	FileFileDownload     = icons.FileFileDownload
	FileCloudDownload    = icons.FileCloudDownload
	ContentAdd           = icons.ContentAdd
	AVPlayArrow          = icons.AVPlayArrow
	AVPause              = icons.AVPause
	CommunicationChatBubble = icons.CommunicationChatBubble
	FileFolder           = icons.FileFolder
	ImageImage           = icons.ImageImage
	FileAttachment       = icons.FileAttachment
	ActionBook           = icons.ActionBook
	ActionCheckCircle    = icons.ActionCheckCircle
)

var ActionDoneIcon, _ = widget.NewIcon(icons.ActionDone)
var ActionDoneAllIcon, _ = widget.NewIcon(icons.ActionDoneAll)
var AlertErrorIcon, _ = widget.NewIcon(icons.AlertErrorOutline)
var SubmitIcon, _ = widget.NewIcon(icons.ContentSend)
var ExpandIcon, _ = widget.NewIcon(icons.NavigationUnfoldMore)
var CollapseIcon, _ = widget.NewIcon(icons.NavigationUnfoldLess)
var VoiceMessageIcon, _ = widget.NewIcon(icons.AVMic)
var AudioCallIcon, _ = widget.NewIcon(icons.CommunicationPhone)
var VideoCallIcon, _ = widget.NewIcon(icons.AVVideoCall)
var MicOffIcon, _ = widget.NewIcon(icons.AVMicOff)
var SettingsIcon, _ = widget.NewIcon(icons.ActionSettings)
var PhotoLibraryIcon, _ = widget.NewIcon(icons.ImagePhotoLibrary)
var ImageBrokenIcon, _ = widget.NewIcon(icons.ImageBrokenImage)
var RefreshIcon, _ = widget.NewIcon(icons.NavigationRefresh)
var SyncIcon, _ = widget.NewIcon(icons.NotificationSync)
var ContentCutIcon, _ = widget.NewIcon(icons.ContentContentCut)
var ContentCopyIcon, _ = widget.NewIcon(icons.ContentContentCopy)
var ContentPasteIcon, _ = widget.NewIcon(icons.ContentContentPaste)
var DownloadIcon, _ = widget.NewIcon(icons.FileFileDownload)
var CloudDownloadIcon, _ = widget.NewIcon(icons.FileCloudDownload)
var AddIcon, _ = widget.NewIcon(icons.ContentAdd)
var PlayIcon, _ = widget.NewIcon(icons.AVPlayArrow)
var PauseIcon, _ = widget.NewIcon(icons.AVPause)
var ChatIcon, _ = widget.NewIcon(icons.CommunicationChatBubble)
var FilesIcon, _ = widget.NewIcon(icons.FileFolder)
var BrowseIcon, _ = widget.NewIcon(Browse)
var ImageIcon, _ = widget.NewIcon(icons.ImageImage)
var UnknownIcon, _ = widget.NewIcon(icons.FileAttachment)
var BookIcon, _ = widget.NewIcon(icons.ActionBook)
var MusicIcon, _ = widget.NewIcon(Audio)
var VideoIcon, _ = widget.NewIcon(Video)
var ApkIcon, _ = widget.NewIcon(Apk)
var FileExportIcon, _ = widget.NewIcon(FileExport)
var CheckCircleIcon, _ = widget.NewIcon(icons.ActionCheckCircle)
