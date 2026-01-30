package core

import (
	"bytes"
	"encoding/json"
	"errors"
	"image"
	"image/gif"
	"io"
	"log"
	"net"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"
)

type FileWriter struct {
	FileId     chan uint32 // finished file id
	Wrq        chan WriteReq
	FileData   chan Data
	OnComplete func(req WriteReq)
	wrq        map[uint32]WriteReq
	fileData   map[uint32][]Data
}

func (f *FileWriter) Loop() {
	for {
		select {
		// last block received, file transfer finished
		case id := <-f.FileId:
			req := f.wrq[id]
			filePath := GetPath(req.UUID, req.Filename)
			write(filePath, f.fileData[id])
			delete(f.wrq, id)
			delete(f.fileData, id)
			if f.OnComplete != nil {
				// for example, reload avatar
				f.OnComplete(req)
			}
			DefaultClient.FileMessages <- req
		// transfer start
		case req := <-f.Wrq:
			f.wrq[req.FileId] = req
			// remove before append
			RemoveFile(GetPath(req.UUID, req.Filename))
		case data := <-f.FileData:
			if !f.isFile(data.FileId) {
				continue
			}
			req := f.wrq[data.FileId]
			f.fileData[data.FileId] = append(f.fileData[data.FileId], data)
			// received ~100kb
			if len(f.fileData[data.FileId]) >= 70 {
				d := write(GetPath(req.UUID, req.Filename), f.fileData[data.FileId])
				if d != nil {
					// not consecutive, store for later use
					f.fileData[data.FileId] = d
				} else {
					f.fileData[data.FileId] = make([]Data, 0)
				}
			}
		}
	}

}

func (f *FileWriter) isFile(fileId uint32) bool {
	return f.wrq[fileId].FileId == fileId
}

func writeTo(filePath string, data []Data) {
	// os.O_CREATE: 如果文件不存在，则创建文件
	// os.O_WRONLY: 以只写模式打开文件
	file, err := os.OpenFile(filePath, os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Printf("error opening file: %v", err)
	}
	defer file.Close()

	readers := make([]io.Reader, 0, len(data))
	for _, d := range data {
		//log.Printf("block: %v", d.Block)
		readers = append(readers, d.Payload)
	}
	multiReader := io.MultiReader(readers...)
	_, err = file.Seek(int64((data[0].Block-1)*BlockSize), 0)
	if err != nil {
		log.Printf("seeking to block %d failed: %v", data[0].Block, err)
	}
	// 使用io.Copy将multiReader的内容写入文件
	if _, err := io.Copy(file, multiReader); err != nil {
		log.Printf("error writing to file: %v", err)
	}
}

type Client struct {
	UUID           string
	ConfigName     string `json:"-"`
	Nickname       string
	SignedMessages chan SignedMessage `json:"-"`
	FileMessages   chan WriteReq      `json:"-"`
	Status         chan struct{}      `json:"-"`
	Connected      bool               `json:"-"`
	MessageCounter uint32
	SyncFunc       func() `json:"-"`
	Sign           string
	ServerAddr     string
	SAddr          net.Addr      `json:"-"`
	Retries        uint8         // the number of times to retry a failed transmission
	Timeout        time.Duration // the duration to wait for an acknowledgement
	fileWriter     *FileWriter
	files          []WriteReq
	conn           net.PacketConn
	audioMetaInfo
}

type audioMetaInfo struct {
	audioMap      map[uint16]WriteReq
	audioReceiver map[uint16][]WriteReq
	AudioData     chan Data `json:"-"`
	lock          sync.Mutex
}

func (a *audioMetaInfo) addAudioStream(wrq WriteReq) {
	a.audioMap[GetHigh16(wrq.FileId)] = wrq
}

func (a *audioMetaInfo) isAudio(fileId uint32) bool {
	audioId := a.decodeAudioId(fileId)
	return a.decodeAudioId(a.audioMap[audioId].FileId) == audioId
}

func (a *audioMetaInfo) decodeAudioId(fileId uint32) uint16 {
	return GetHigh16(fileId)
}

func (a *audioMetaInfo) addAudioReceiver(fileId uint16, wrq WriteReq) {
	a.audioReceiver[fileId] = slices.DeleteFunc(a.audioReceiver[fileId], func(w WriteReq) bool {
		return w.UUID == wrq.UUID
	})
	a.audioReceiver[fileId] = append(a.audioReceiver[fileId], wrq)
}

func (a *audioMetaInfo) deleteAudioReceiver(fileId uint16, UUID string) {
	a.lock.Lock()
	defer a.lock.Unlock()
	a.audioReceiver[fileId] = slices.DeleteFunc(a.audioReceiver[fileId], func(w WriteReq) bool {
		return w.UUID == UUID
	})
}

func (a *audioMetaInfo) cleanupAudioResource(fileId uint16) bool {
	if len(a.audioReceiver[fileId]) == 0 {
		delete(a.audioReceiver, fileId)
		delete(a.audioMap, fileId)
		return true
	}
	return false
}

func (c *Client) Ready() {
	if c.Status != nil {
		<-c.Status
	}
}

func (c *Client) HandleFileWith(callback func(req WriteReq)) {
	c.fileWriter.OnComplete = callback
}

func (c *Client) SyncIcon(img image.Image) error {
	return c.sendImage(img, OpSyncIcon, "icon.png")
}

func (c *Client) SyncGif(gifImg *gif.GIF) error {
	return c.sendGif(gifImg, OpSyncIcon, "icon.gif")
}

func (c *Client) SendImage(img image.Image, filename string) error {
	if filepath.Ext(filename) == ".webp" {
		filename = strings.TrimSuffix(filepath.Base(filename), ".webp") + ".png"
	}
	return c.sendImage(img, OpSendImage, filename)
}

func (c *Client) sendImage(img image.Image, code OpCode, filename string) error {
	buf := new(bytes.Buffer)
	err := EncodeImg(buf, filename, img)
	if err != nil {
		return err
	}

	return c.sendFile(bytes.NewReader(buf.Bytes()), code, filename, 0, 0)
}

func (c *Client) SendGif(GIF *gif.GIF, filename string) error {
	return c.sendGif(GIF, OpSendGif, filename)
}

func (c *Client) sendGif(GIF *gif.GIF, code OpCode, filename string) error {
	buf := new(bytes.Buffer)
	err := gif.EncodeAll(buf, GIF)
	if err != nil {
		return err
	}
	return c.sendFile(bytes.NewReader(buf.Bytes()), code, filename, 0, 0)
}

func (c *Client) SendVoice(filename string, duration uint64) error {
	r, err := os.Open(GetDataPath(filename))
	if err != nil {
		return err
	}
	defer r.Close()
	return c.sendFile(r, OpSendVoice, filename, 0, duration)
}

func (c *Client) SendAudioPacket(fileId uint32, blockId uint32, packet []byte) error {
	data := Data{FileId: fileId, Block: blockId, Payload: bytes.NewReader(packet)}
	pkt, err := data.Marshal()
	if err != nil {
		return err
	}
	_, err = c.conn.WriteTo(pkt, c.SAddr)
	if err != nil {
		return err
	}
	return nil
}

func (c *Client) MakeAudioCall(fileId uint32) error {
	c.addAudioStream(WriteReq{Code: OpAudioCall, FileId: fileId, UUID: c.FullID()})
	return c.sendReq(OpAudioCall, fileId)
}

func (c *Client) EndAudioCall(fileId uint32) error {
	audioId := GetHigh16(fileId)
	c.deleteAudioReceiver(audioId, c.FullID())
	c.cleanupAudioResource(audioId)
	c.FileMessages <- WriteReq{Code: OpEndAudioCall, FileId: fileId, UUID: c.FullID()}
	return c.sendReq(OpEndAudioCall, fileId)
}

func (c *Client) AcceptAudioCall(fileId uint32) error {
	return c.sendReq(OpAcceptAudioCall, fileId)
}

func (c *Client) sendReq(code OpCode, fileId uint32) error {
	conn, err := net.Dial("udp", c.ServerAddr)
	if err != nil {
		return err
	}
	defer func() { _ = conn.Close() }()
	wrq := WriteReq{Code: code, FileId: fileId, UUID: c.FullID()}
	pkt, err := wrq.Marshal()
	if err != nil {
		return err
	}
	pktBuf := make([]byte, DatagramSize)
	_, err = c.sendPacket(conn, pktBuf, pkt, 0)
	if err != nil {
		return err
	}
	return nil
}

func (c *Client) sendFile(reader io.Reader, code OpCode,
	filename string, size uint64, duration uint64) error {
	conn, err := net.Dial("udp", c.ServerAddr)
	if err != nil {
		return err
	}
	defer func() { _ = conn.Close() }()

	hash := Hash(unsafe.Pointer(&reader))
	log.Printf("file id: %v", hash)

	pktBuf := make([]byte, DatagramSize)

	wrq := WriteReq{Code: code, FileId: hash, UUID: c.FullID(),
		Filename: filename, Size: size, Duration: duration}
	pkt, err := wrq.Marshal()
	if err != nil {
		return err
	}
	_, err = c.sendPacket(conn, pktBuf, pkt, 0)
	if err != nil {
		return err
	}

	data := Data{FileId: wrq.FileId, Payload: reader}
	err = c.sendData(conn, pktBuf, data)
	if err != nil {
		return err
	}
	return nil
}

func (c *Client) sendData(conn net.Conn, pktBuf []byte, data Data) error {
	for n := DatagramSize; n == DatagramSize; {
		pkt, err := data.Marshal()
		if err != nil {
			log.Printf("[%s] marshal failed: %v", "icon", err)
		}
		n, err = c.sendPacket(conn, pktBuf, pkt, data.Block)
		if err != nil {
			return err
		}
	}
	return nil
}

func (c *Client) SendText(text string) error {
	conn, err := net.Dial("udp", c.ServerAddr)
	if err != nil {
		log.Printf("[%s] dial failed: %v", c.ServerAddr, err)
	}
	defer func() { _ = conn.Close() }()

	msg := SignedMessage{Sign: Sign{c.Sign, c.FullID()}, Payload: []byte(text)}
	pkt, err := msg.Marshal()
	if err != nil {
		log.Printf("[%s] marshal failed: %v", text, err)
	}

	c.MessageCounter++
	buf := make([]byte, DatagramSize)
	_, err = c.sendPacket(conn, buf, pkt, 0)
	return err
}

func (c *Client) FullID() string {
	return c.Nickname + c.UUID
}

func (c *Client) ListenAndServe(addr string) {
	conn, err := net.ListenPacket("udp", addr)
	if err != nil {
		log.Printf("[%s] dial failed: %v", addr, err)
	}
	c.conn = conn
	defer func() { _ = conn.Close() }()

	// init
	c.init()

	go c.fileWriter.Loop()

	if c.Status != nil {
		close(c.Status)
	}

	c.SAddr, err = net.ResolveUDPAddr("udp", c.ServerAddr)
	go func() {
		// auto reconnect in case of server down
		var threshold uint32 = 5
		for {
			c.SendSign()
			sentEnoughMessages := c.MessageCounter > threshold
			if sentEnoughMessages && c.SyncFunc != nil {
				c.MessageCounter = 0
				threshold++
				c.SyncFunc()
			}
			time.Sleep(30 * time.Second)
		}
	}()

	log.Printf("Listening on %s ...\n", conn.LocalAddr())
	c.serve(conn)
}

func (c *Client) init() {
	if c.Retries == 0 {
		c.Retries = 3
	}

	if c.Timeout == 0 {
		c.Timeout = 6 * time.Second
	}

	Mkdir(GetDir(c.FullID()))

	c.SignedMessages = make(chan SignedMessage, 100)
	c.FileMessages = make(chan WriteReq, 100)
	c.audioMap = make(map[uint16]WriteReq)
	c.audioReceiver = make(map[uint16][]WriteReq)
	c.AudioData = make(chan Data, 100)

	c.fileWriter = &FileWriter{Wrq: make(chan WriteReq), FileData: make(chan Data),
		FileId: make(chan uint32), wrq: make(map[uint32]WriteReq), fileData: make(map[uint32][]Data)}
}

func (c *Client) SendSign() {
	sign := Sign{c.Sign, c.FullID()}
	pkt, err := sign.Marshal()
	if err != nil {
		log.Printf("[%s] marshal failed: %v", c.Sign, err)
	}
	_, err = c.conn.WriteTo(pkt, c.SAddr)
	if err != nil {
		log.Printf("[%s] write failed: %v", c.ServerAddr, err)
		return
	}
}

func (c *Client) serve(conn net.PacketConn) {

	for {
		buf := make([]byte, DatagramSize)
		_ = conn.SetReadDeadline(time.Now().Add(c.Timeout))
		n, addr, err := conn.ReadFrom(buf)
		if err != nil {
			var nErr net.Error
			if errors.As(err, &nErr) && nErr.Timeout() {
				//log.Printf("receive text timeout")
			}
			if errors.Is(err, syscall.ECONNREFUSED) {
				c.Connected = false
				log.Printf("[%s] connection refused", c.ServerAddr)
			}
			//log.Printf("[%s] receive text: %v", c.ServerAddr, err)
			continue
		}
		go c.handle(buf[:n], conn, addr)
	}
}

func (c *Client) handle(buf []byte, conn net.PacketConn, addr net.Addr) {
	var (
		ack  Ack
		msg  SignedMessage
		data Data
		wrq  WriteReq
	)
	switch {
	case ack.Unmarshal(buf) == nil:
		if ack.SrcOp == OpSign {
			c.Connected = true
		}
	case msg.Unmarshal(buf) == nil:
		s := string(msg.Payload)
		log.Printf("received text [%s] from [%s]\n", s, msg.Sign.UUID)
		c.SignedMessages <- msg
		c.ack(conn, addr, OpSignedMSG, 0)
	case wrq.Unmarshal(buf) == nil:
		c.ack(conn, addr, wrq.Code, 0)
		audioId := c.decodeAudioId(wrq.FileId)
		switch wrq.Code {
		case OpAudioCall:
			c.addAudioStream(wrq)
			fallthrough
		case OpAcceptAudioCall:
			c.addAudioReceiver(audioId, wrq)
			c.FileMessages <- wrq
		case OpEndAudioCall:
			c.deleteAudioReceiver(audioId, wrq.UUID)
			cancel := c.audioMap[audioId].UUID == wrq.UUID
			cleanup := c.cleanupAudioResource(audioId)
			if cancel {
				wrq.FileId = 0
			}
			if cleanup {
				c.FileMessages <- wrq
			}
		default:
			c.addFile(wrq)
		}
	case data.Unmarshal(buf) == nil:
		if c.isAudio(data.FileId) {
			c.AudioData <- data
			return
		}
		if c.fileWriter.isFile(data.FileId) {
			c.handleFileData(conn, addr, data, len(buf))
		}
	}
}

func (c *Client) handleFileData(conn net.PacketConn, addr net.Addr, data Data, n int) {
	c.ack(conn, addr, OpData, data.Block)
	c.fileWriter.FileData <- data
	if n < DatagramSize {
		c.fileWriter.FileId <- data.FileId
		log.Printf("file id: [%d] received", data.FileId)
	}
}

func (c *Client) addFile(wrq WriteReq) {
	c.files = append(c.files, wrq)
	c.fileWriter.Wrq <- wrq
}

func (c *Client) ack(conn net.PacketConn, clientAddr net.Addr, code OpCode, block uint32) {
	ack := Ack{SrcOp: code, Block: block}
	pkt, err := ack.Marshal()
	_, err = conn.WriteTo(pkt, clientAddr)
	if err != nil {
		log.Printf("[%s] write failed: %v", clientAddr, err)
		return
	}
}

func (c *Client) SetNickName(nickname string) {
	c.Nickname = nickname
}

func (c *Client) SetSign(sign string) {
	c.Sign = sign
}

func (c *Client) SetServerAddr(addr string) {
	c.ServerAddr = addr
	c.SAddr, _ = net.ResolveUDPAddr("udp", addr)
}

func (c *Client) sendPacket(conn net.Conn, buf []byte, bytes []byte, block uint32) (int, error) {
	var ackPkt Ack
RETRY:
	for i := c.Retries; i > 0; i-- {
		n, err := conn.Write(bytes)
		if err != nil {
			log.Printf("[%s] write failed: %v", c.ServerAddr, err)
			return 0, err
		}

		// wait for the Server's ACK packet
		_ = conn.SetReadDeadline(time.Now().Add(c.Timeout))
		_, err = conn.Read(buf)

		if err != nil {
			if nErr, ok := err.(net.Error); ok && nErr.Timeout() {
				log.Printf("waiting for ACK timeout")
				continue RETRY
			}
			if errors.Is(err, syscall.ECONNREFUSED) {
				c.Connected = false
				log.Printf("[%s] connection refused", c.ServerAddr)
				return 0, err
			}
			log.Printf("[%s] waiting for ACK: %v", c.ServerAddr, err)
			continue
		}

		switch {
		case ackPkt.Unmarshal(buf[:n]) == nil:
			if block == 0 || ackPkt.Block == block {
				return n, nil
			}
		default:
			log.Printf("[%s] bad packet", c.ServerAddr)
		}
	}
	return 0, errors.New("exhausted retries")
}

func Load(configName string) *Client {
	filePath := getFilePath(configName)
	_, err := os.Stat(filePath)
	if os.IsNotExist(err) {
		return nil
	}
	file, err := os.Open(filePath)
	if err != nil {
		log.Printf("[%s] open file failed: %v", filePath, err)
		return nil
	}
	defer file.Close()
	var c Client
	decoder := json.NewDecoder(file)
	err = decoder.Decode(&c)
	if err != nil {
		log.Printf("[%s] decode failed: %v", filePath, err)
		return nil
	}
	return &c
}

func (c *Client) Store() {
	filePath := getFilePath(c.ConfigName)
	Mkdir(GetDataDir())
	file, err := os.Create(filePath)
	if err != nil {
		log.Printf("[%s] create file failed: %v", filePath, err)
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	err = encoder.Encode(&c)
	if err != nil {
		log.Printf("[%s] encode file failed: %v", filePath, err)
	}
}

var DefaultClient *Client
