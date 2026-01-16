package view

import "gioui.org/layout"

var VoiceMode = false

func SwitchBetweenTextAndVoice(voiceMessage *IconButton) func(gtx layout.Context) {
	return func(gtx layout.Context) {
		iconStackAnimation.Disappear(gtx.Now)
		VoiceMode = !VoiceMode
		if VoiceMode {
			voiceMessage.Icon = chatIcon
		} else {
			voiceMessage.Icon = voiceMessageIcon
		}
	}
}
