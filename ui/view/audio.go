package view

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"math"
	"rtc/assets/fonts"
	"rtc/internal/audio"
	"time"
	"unsafe"

	"gioui.org/x/component"
	"github.com/CoyAce/opus"
	"github.com/CoyAce/opus/ogg"
	"github.com/CoyAce/wi"
)

var audioStackAnimation = component.VisibilityAnimation{
	Duration: time.Millisecond * 100,
	State:    component.Invisible,
	Started:  time.Time{},
}

var (
	audioMode                 Mode
	audioId                   uint16
	timestamp                 uint16
	mute                      bool
	micOffButton              = &IconButton{Theme: fonts.DefaultTheme, Icon: micOffIcon, Enabled: true, Hidden: true}
	audioMakeButton           = &IconButton{Theme: fonts.DefaultTheme, Icon: audioCallIcon, Enabled: true}
	audioAcceptButton         = &IconButton{Theme: fonts.DefaultTheme, Icon: audioCallIcon, Enabled: true, Mode: Accept}
	captureCtx, captureCancel = context.WithCancel(context.Background())
	playbackCancels           []context.CancelFunc
	players                   = make(map[uint16]chan *bytes.Buffer)
	enhancer                  = audio.DefaultAudioEnhancer()
)

type BlockId uint32

func (b *BlockId) next() uint32 {
	*b++
	if *b == math.MaxUint32 {
		*b = 0
	}
	return uint32(*b)
}

func encodeAudioId() uint32 {
	return wi.CombineUint32(audioId, timestamp)
}

func generateAudioId() uint32 {
	audioId = uint16(wi.Hash(unsafe.Pointer(&struct{}{})))
	timestamp = uint16(time.Now().Unix())
	return wi.CombineUint32(audioId, timestamp)
}

func NewAudioIconStack(streamConfig audio.StreamConfig) *IconStack {
	audioAcceptButton.OnClick = acceptAudioCall(streamConfig)
	var audioDeclineButton = &IconButton{Theme: fonts.DefaultTheme, Icon: audioCallIcon, Enabled: true, Mode: Decline}
	audioDeclineButton.OnClick = func() {
		audioMakeButton.Hidden = false
		resetMuteButton()
		audioStackAnimation.Disappear(time.Now())
		time.AfterFunc(audioStackAnimation.Duration, func() {
			audioAcceptButton.Hidden = false
		})
		captureCancel()
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
			err := wi.DefaultClient.SendText(text + "了语音通话")
			if err != nil {
				log.Printf("audio call failed: %v", err)
			}
			err = wi.DefaultClient.EndAudioCall(encodeAudioId())
			if err != nil {
				log.Printf("audio call error: %v", err)
			}
		}()
	}
	micOffButton.OnClick = func() {
		toggleMuteButton()
	}
	return &IconStack{
		Sticky:              true,
		Theme:               fonts.DefaultTheme,
		VisibilityAnimation: &audioStackAnimation,
		IconButtons: []*IconButton{
			audioAcceptButton,
			audioDeclineButton,
			micOffButton,
		},
	}
}

func acceptAudioCall(streamConfig audio.StreamConfig) func() {
	return func() {
		audioAcceptButton.Hidden = true
		audioMode = Accept
		timestamp = uint16(time.Now().Unix())
		encodedAudioId := encodeAudioId()
		go func() {
			err := wi.DefaultClient.SendText("接受了语音通话")
			if err != nil {
				log.Printf("audio call error: %v", err)
			}
			err = wi.DefaultClient.AcceptAudioCall(encodedAudioId)
			if err != nil {
				log.Printf("audio call error: %v", err)
			}
			PostAudioCallAccept(streamConfig)
		}()
	}
}

func ShowIncomingCall(wrq wi.WriteReq) {
	if audioMode == Accept {
		return
	}
	audioId = wi.GetHigh16(wrq.FileId)
	audioMode = Decline
	audioAcceptButton.Hidden = false
	audioMakeButton.Hidden = true
	audioStackAnimation.Appear(time.Now())
}

func EndIncomingCall(cancel bool) {
	if audioMode != Accept && !cancel {
		return
	}
	audioMode = None
	audioMakeButton.Hidden = false
	audioAcceptButton.Hidden = true
	resetMuteButton()
	audioStackAnimation.Disappear(time.Now())
	captureCancel()
}

func toggleMuteButton() {
	mute = !mute
	micOffButton.Mode = None
	if mute {
		micOffButton.Mode = Decline
	}
}

func resetMuteButton() {
	micOffButton.Hidden = true
	micOffButton.Mode = None
	mute = false
}

func initMuteButton() {
	micOffButton.Hidden = false
	micOffButton.Mode = None
	mute = false
}

func PostAudioCallAccept(streamConfig audio.StreamConfig) {
	audioMode = Accept
	initMuteButton()
	audioChunks := make(chan *bytes.Buffer, 10)
	captureCtx, captureCancel = context.WithCancel(context.Background())
	writer := audio.NewChunkWriter(captureCtx, audioChunks)
	enhancer.Initialize()
	go func() {
		streamConfig.PeriodSizeInFrames = 120
		streamConfig.Periods = 2
		if err := audio.Capture(captureCtx, writer, streamConfig); err != nil {
			if errors.Is(err, context.Canceled) {
				return
			}
			log.Printf("capture audio failed, %s", err)
			captureCancel()
		}
	}()
	go func() {
		enc, err := opus.NewEncoder(ogg.SampleRate, 1, opus.AppVoIP)
		if err != nil {
			log.Printf("create audio encoder failed, %s", err)
			return
		}
		fileId := encodeAudioId()
		blockId := BlockId(0)
		for {
			var cur *bytes.Buffer
			select {
			// Received from AudioChunkWriter
			case cur = <-audioChunks:
			case <-captureCtx.Done():
				for _, cancel := range playbackCancels {
					cancel()
				}
				playbackCancels = playbackCancels[:0]
				players = make(map[uint16]chan *bytes.Buffer)
				return
			}
			data := make([]byte, ogg.FrameSize)
			start := time.Now()
			processAudio, err := enhancer.ProcessAudio(audio.Int16BytesToFloat32(cur.Bytes()))
			cost := time.Since(start)
			if err != nil {
				log.Printf("enhancer process audio failed, %s", err)
				continue
			}
			n, err := enc.Encode(audio.Float32ToInt16(processAudio), data)
			if err != nil {
				log.Printf("audio encode failed, %s", err)
				continue
			}
			if mute {
				continue
			}
			err = wi.DefaultClient.SendAudioPacket(fileId, blockId.next(), data[:n])
			if err != nil {
				log.Printf("audio call error: %v", err)
			}
			metrics := enhancer.GetMetrics()
			fmt.Printf("ERLE: %.1f divergent_filter_fraction: %.1f Delay: %d Cost: %v\n",
				metrics.Stats.EchoReturnLossEnhancement, metrics.Stats.DivergentFilterFraction, metrics.Stats.DelayMs, cost)
		}
	}()
}

func ConsumeAudioData(streamConfig audio.StreamConfig) {
	dec, err := opus.NewDecoder(ogg.SampleRate, 1)
	if err != nil {
		log.Printf("create audio decoder failed, %s", err)
	}
	for data := range wi.DefaultClient.AudioData {
		if captureCtx.Err() != nil {
			continue
		}
		identity := wi.GetLow16(data.FileId)
		//log.Printf("fileId:%d, timestamp: %d, block id %d",
		//	wi.GetHigh16(data.FileId), identity, data.Block)
		if players[identity] == nil {
			pcmChunks := make(chan *bytes.Buffer, 15000) // 50 * 300(s) = 5(min)
			players[identity] = pcmChunks
			go newPlayer(pcmChunks, streamConfig)
		}
		packet, err := io.ReadAll(data.Payload)
		if err != nil {
			log.Printf("read audio packet failed, %s", err)
			continue
		}
		buf := make([]int16, ogg.FrameSize)
		n, err := dec.Decode(packet, buf)
		if err != nil {
			log.Printf("decode audio packet failed, %s", err)
			continue
		}
		enhancer.AddFarEnd(buf[:n])
		select {
		case players[identity] <- bytes.NewBuffer(ogg.ToBytes(buf[:n])):
		default:
			log.Printf("buffer full, packet discarded")
		}
	}
}

func newPlayer(pcmChunks <-chan *bytes.Buffer, streamConfig audio.StreamConfig) {
	playbackCtx, playbackCancel := context.WithCancel(context.Background())
	playbackCancels = append(playbackCancels, playbackCancel)
	reader := audio.NewChunkReader(playbackCtx, pcmChunks)
	if err := audio.Playback(playbackCtx, reader, streamConfig); err != nil && !errors.Is(err, io.EOF) {
		log.Printf("audio playback: %w", err)
	}
}

func MakeAudioCall(audioButton *IconButton) func() {
	return func() {
		audioMode = None
		audioButton.Hidden = true
		audioAcceptButton.Hidden = true
		time.AfterFunc(iconStackAnimation.Duration, func() {
			audioStackAnimation.Appear(time.Now())
		})
		go func() {
			err := wi.DefaultClient.SendText("发起了语音通话")
			if err != nil {
				log.Printf("audio call error: %v", err)
			}

			err = wi.DefaultClient.MakeAudioCall(generateAudioId())
			if err != nil {
				log.Printf("audio call err: %v", err)
				audioStackAnimation.Disappear(time.Now())
			}
		}()
	}
}
