package view

var VoiceMode = false

func SwitchBetweenTextAndVoice(voiceMessage *IconButton) func() {
	return func() {
		VoiceMode = !VoiceMode
		if VoiceMode {
			voiceMessage.Icon = chatIcon
		} else {
			voiceMessage.Icon = voiceMessageIcon
		}
	}
}
