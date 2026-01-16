package view

import (
	"log"
	"rtc/assets/fonts"
	"rtc/core"
	"rtc/internal/audio"
	"time"

	"gioui.org/layout"
	"gioui.org/x/component"
)

var audioStackAnimation = component.VisibilityAnimation{
	Duration: time.Millisecond * 100,
	State:    component.Invisible,
	Started:  time.Time{},
}

var audioMode Mode

var audioId uint32
var audioMakeButton = &IconButton{Theme: fonts.DefaultTheme, Icon: audioCallIcon, Enabled: true}
var audioAcceptButton = &IconButton{Theme: fonts.DefaultTheme, Icon: audioCallIcon, Enabled: true, Mode: Accept}

func NewAudioIconStack(streamConfig audio.StreamConfig) *IconStack {
	audioAcceptButton.OnClick = acceptAudioCall()
	var audioDeclineButton = &IconButton{Theme: fonts.DefaultTheme, Icon: audioCallIcon, Enabled: true, Mode: Decline}
	audioDeclineButton.OnClick = func(gtx layout.Context) {
		audioMakeButton.Hidden = false
		audioStackAnimation.Disappear(gtx.Now)
		time.AfterFunc(audioStackAnimation.Duration, func() {
			audioAcceptButton.Hidden = false
		})
		go func() {
			var text string
			switch audioMode {
			case None:
				text = "取消"
			case Accept:
				text = "结束"
				audioMode = None
			case Decline:
				text = "拒绝"
				audioMode = None
			default:
			}
			err := core.DefaultClient.SendText(text + "了语音通话")
			if err != nil {
				log.Printf("audio call failed: %v", err)
			}
			err = core.DefaultClient.EndAudioCall(audioId)
			if err != nil {
				log.Printf("audio call error: %v", err)
			}
		}()
	}
	return &IconStack{Theme: fonts.DefaultTheme,
		VisibilityAnimation: &audioStackAnimation,
		IconButtons: []*IconButton{
			audioAcceptButton,
			audioDeclineButton,
		},
	}
}

func acceptAudioCall() func(gtx layout.Context) {
	return func(gtx layout.Context) {
		audioAcceptButton.Hidden = true
		audioMode = Accept
		go func() {
			err := core.DefaultClient.SendText("接受了语音通话")
			if err != nil {
				log.Printf("audio call error: %v", err)
			}
			err = core.DefaultClient.AcceptAudioCall(audioId)
			if err != nil {
				log.Printf("audio call error: %v", err)
			}
			AcceptAudioCall()
		}()
	}
}

func ShowIncomingCall(wrq core.WriteReq) {
	if audioMode == Accept {
		return
	}
	audioId = wrq.FileId
	audioMode = Decline
	audioAcceptButton.Hidden = false
	audioMakeButton.Hidden = true
	audioStackAnimation.Appear(time.Now())
}

func EndIncomingCall() {
	audioMode = None
	audioAcceptButton.Hidden = true
	audioMakeButton.Hidden = false
	audioStackAnimation.Disappear(time.Now())
}

func AcceptAudioCall() {
	audioMode = Accept
	core.DefaultClient.SendText("audio accepted")
}

func MakeAudioCall(audioButton *IconButton) func(gtx layout.Context) {
	return func(gtx layout.Context) {
		audioMode = None
		audioButton.Hidden = true
		audioAcceptButton.Hidden = true
		time.AfterFunc(iconStackAnimation.Duration, func() {
			audioStackAnimation.Appear(gtx.Now)
		})
		go func() {
			err := core.DefaultClient.SendText("发起了语音通话")
			if err != nil {
				log.Printf("audio call error: %v", err)
			}
			audioId, err = core.DefaultClient.MakeAudioCall()
			if err != nil {
				log.Printf("audio call err: %v", err)
				audioStackAnimation.Disappear(gtx.Now)
			}
		}()
	}
}
