package view

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log"
	"rtc/assets/fonts"
	"rtc/core"
	"rtc/internal/audio"
	"time"

	"gioui.org/layout"
	"gioui.org/x/component"
	"github.com/CoyAce/opus"
	"github.com/CoyAce/opus/ogg"
)

var audioStackAnimation = component.VisibilityAnimation{
	Duration: time.Millisecond * 100,
	State:    component.Invisible,
	Started:  time.Time{},
}

var (
	audioMode                 Mode
	audioId                   uint32
	audioMakeButton           = &IconButton{Theme: fonts.DefaultTheme, Icon: audioCallIcon, Enabled: true}
	audioAcceptButton         = &IconButton{Theme: fonts.DefaultTheme, Icon: audioCallIcon, Enabled: true, Mode: Accept}
	bytesOf20ms               = ogg.FrameSize * ogg.Channels * 2
	bytesOf100ms              = bytesOf20ms * 5
	captureCtx, captureCancel = context.WithCancel(context.Background())
	playbackCancels           []context.CancelFunc
	players                   = make(map[uint32]chan *bytes.Buffer)
)

func NewAudioIconStack(streamConfig audio.StreamConfig) *IconStack {
	audioAcceptButton.OnClick = acceptAudioCall(streamConfig)
	var audioDeclineButton = &IconButton{Theme: fonts.DefaultTheme, Icon: audioCallIcon, Enabled: true, Mode: Decline}
	audioDeclineButton.OnClick = func(gtx layout.Context) {
		audioMakeButton.Hidden = false
		audioStackAnimation.Disappear(gtx.Now)
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

func acceptAudioCall(streamConfig audio.StreamConfig) func(gtx layout.Context) {
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
			PostAudioCallAccept(streamConfig)
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
	if audioMode != Accept {
		return
	}
	audioMode = None
	audioAcceptButton.Hidden = true
	audioMakeButton.Hidden = false
	audioStackAnimation.Disappear(time.Now())
	captureCancel()
}

func PostAudioCallAccept(streamConfig audio.StreamConfig) {
	audioMode = Accept
	audioChunks := make(chan *bytes.Buffer, 10)
	writer := newChunkWriter(captureCtx, audioChunks)
	captureCtx, captureCancel = context.WithCancel(context.Background())
	go func() {
		if err := audio.Capture(captureCtx, writer, streamConfig); err != nil {
			if errors.Is(err, context.Canceled) {
				return
			}
			log.Printf("capture audio failed, %s", err)
			captureCancel()
		}
	}()
	go func() {
		enc, err := opus.NewEncoder(ogg.SampleRate, ogg.Channels, opus.AppVoIP)
		if err != nil {
			log.Printf("create audio encoder failed, %s", err)
			return
		}
		pcmProcessor := audio.NewPCMProcessor()
		for {
			var cur *bytes.Buffer
			select {
			// Received from audioChunkWriter
			case cur = <-audioChunks:
			case <-captureCtx.Done():
				return
			}
			data := make([]byte, ogg.FrameSize)
			n, err := enc.Encode(pcmProcessor.Normalize(audio.ToPcmInts(cur.Bytes())), data)
			if err != nil {
				log.Printf("audio encode failed, %s", err)
			}
			err = core.DefaultClient.SendAudioPacket(audioId, data[:n])
			if err != nil {
				log.Printf("audio call error: %v", err)
			}
		}
	}()
	go func() {
		dec, err := opus.NewDecoder(ogg.SampleRate, ogg.Channels)
		if err != nil {
			log.Printf("create audio decoder failed, %s", err)
			return
		}
		for {
			select {
			case data := <-core.DefaultClient.AudioData:
				if players[data.Block] == nil {
					pcmChunks := make(chan *bytes.Buffer, 10)
					players[data.Block] = pcmChunks
					go newPlayer(pcmChunks, streamConfig)
				}
				packet, err := io.ReadAll(data.Payload)
				if err != nil {
					log.Printf("read audio packet failed, %s", err)
					continue
				}
				buf := make([]int16, ogg.FrameSize*int(ogg.Channels))
				n, err := dec.Decode(packet, buf)
				players[data.Block] <- bytes.NewBuffer(audio.ToPcmBytes(buf[:n*ogg.Channels]))
			case <-captureCtx.Done():
				for _, cancel := range playbackCancels {
					cancel()
				}
				return
			}
		}
	}()
}

func newPlayer(pcmChunks <-chan *bytes.Buffer, streamConfig audio.StreamConfig) {
	playbackCtx, playbackCancel := context.WithCancel(context.Background())
	playbackCancels = append(playbackCancels, playbackCancel)
	reader := newChunkReader(playbackCtx, pcmChunks)
	if err := audio.Playback(playbackCtx, reader, streamConfig); err != nil && !errors.Is(err, io.EOF) {
		log.Printf("audio playback: %w", err)
	}
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

type audioChunkWriter struct {
	ctx     context.Context
	ready   chan<- *bytes.Buffer
	current *bytes.Buffer
}

func newChunkWriter(ctx context.Context, ready chan<- *bytes.Buffer) *audioChunkWriter {
	buf := new(bytes.Buffer)
	buf.Grow(bytesOf100ms) // 100ms
	return &audioChunkWriter{
		ctx:     ctx,
		ready:   ready,
		current: buf,
	}
}

func (rbw *audioChunkWriter) Write(p []byte) (n int, err error) {
	if rbw.current.Len() >= bytesOf20ms {
		buf := new(bytes.Buffer)
		buf.Grow(bytesOf20ms)
		_, err = io.CopyN(buf, rbw.current, int64(bytesOf20ms))
		select {
		case rbw.ready <- buf:
			break
		case <-rbw.ctx.Done():
			return 0, rbw.ctx.Err()
		}
		rbw.current = bytes.NewBuffer(rbw.current.Bytes())
	}

	return rbw.current.Write(p)
}

func newChunkReader(ctx context.Context, ready <-chan *bytes.Buffer) *audioChunkReader {
	buf := new(bytes.Buffer)
	buf.Grow(bytesOf100ms) // 100ms
	return &audioChunkReader{
		ctx:     ctx,
		ready:   ready,
		current: buf,
	}
}

type audioChunkReader struct {
	ctx     context.Context
	ready   <-chan *bytes.Buffer
	current *bytes.Buffer
}

func (rbr *audioChunkReader) Read(p []byte) (n int, err error) {
	if rbr.current.Len() >= len(p) {
		return rbr.current.Read(p)
	}
	n, err = rbr.current.Read(p)
	rbr.current.Reset()
	select {
	case buf := <-rbr.ready:
		_, err = io.Copy(rbr.current, buf)
		return
	case <-rbr.ctx.Done():
		return 0, rbr.ctx.Err()
	}
}
