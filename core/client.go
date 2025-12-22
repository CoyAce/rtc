package core

import (
	"bytes"
	"encoding/json"
	"errors"
	"image"
	"io"
	"log"
	"net"
	"os"
	"path/filepath"
	"strings"
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
			filePath := GetFilePath(req.UUID, req.Filename)
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
			RemoveFile(GetFilePath(req.UUID, req.Filename))
		case data := <-f.FileData:
			f.fileData[data.FileId] = append(f.fileData[data.FileId], data)
			// received ~100kb
			if len(f.fileData[data.FileId]) >= 70 {
				req := f.wrq[data.FileId]
				d := write(GetFilePath(req.UUID, req.Filename), f.fileData[data.FileId])
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

func appendTo(filePath string, data []Data) {
	// 使用os.O_APPEND, os.O_CREATE, os.O_WRONLY标志
	// os.O_APPEND: 追加模式，写入的数据会被追加到文件尾部
	// os.O_CREATE: 如果文件不存在，则创建文件
	// os.O_WRONLY: 以只写模式打开文件
	file, err := os.OpenFile(filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
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
	Conn           net.PacketConn     `json:"-"`
	Connected      bool               `json:"-"`
	MessageCounter uint32
	SyncFunc       func() `json:"-"`
	Sign           string
	ServerAddr     string
	SAddr          net.Addr      `json:"-"`
	Retries        uint8         // the number of times to retry a failed transmission
	Timeout        time.Duration // the duration to wait for an acknowledgement
	fileWriter     *FileWriter
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

func (c *Client) SendImage(img image.Image, filename string) error {
	if filepath.Ext(filename) == ".webp" {
		filename = strings.TrimSuffix(filepath.Base(filename), ".webp") + ".png"
	}
	return c.sendImage(img, OpSendImage, filename)
}

func (c *Client) sendImage(img image.Image, code OpCode, filename string) error {
	conn, err := net.Dial("udp", c.ServerAddr)
	if err != nil {
		log.Printf("[%s] dial failed: %v", c.ServerAddr, err)
	}
	defer func() { _ = conn.Close() }()

	buf := new(bytes.Buffer)
	err = Encode(img, filename, buf)
	if err != nil {
		return err
	}
	hash := Hash(unsafe.Pointer(&buf))
	log.Printf("icon file id: %v", hash)

	pktBuf := make([]byte, DatagramSize)

	wrq := WriteReq{code, hash, c.FullID(), filename}
	pkt, err := wrq.Marshal()
	if err != nil {
		log.Printf("[%v] write request marshal failed: %v", wrq, err)
	}
	_, err = c.sendPacket(conn, pktBuf, pkt, 0)
	if err != nil {
		return err
	}

	data := Data{FileId: wrq.FileId, Payload: bytes.NewReader(buf.Bytes())}
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
	c.Conn = conn
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

	c.fileWriter = &FileWriter{Wrq: make(chan WriteReq), FileData: make(chan Data),
		FileId: make(chan uint32), wrq: make(map[uint32]WriteReq, 0), fileData: make(map[uint32][]Data)}
}

func (c *Client) SendSign() {
	sign := Sign{c.Sign, c.FullID()}
	pkt, err := sign.Marshal()
	if err != nil {
		log.Printf("[%s] marshal failed: %v", c.Sign, err)
	}
	_, err = c.Conn.WriteTo(pkt, c.SAddr)
	if err != nil {
		log.Printf("[%s] write failed: %v", c.ServerAddr, err)
		return
	}
}

func (c *Client) serve(conn net.PacketConn) {
	var (
		ack  Ack
		msg  SignedMessage
		data Data
		wrq  WriteReq
	)
	buf := make([]byte, DatagramSize)

	for {
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

		switch {
		case ack.Unmarshal(buf[:n]) == nil:
			if ack.SrcOp == OpSign {
				c.Connected = true
			}
			continue
		case msg.Unmarshal(buf[:n]) == nil:
			s := string(msg.Payload)
			log.Printf("received text [%s] from [%s]\n", s, msg.Sign.UUID)
			c.SignedMessages <- msg
			c.ack(conn, addr, OpSignedMSG, 0)
		case wrq.Unmarshal(buf[:n]) == nil:
			c.ack(conn, addr, wrq.Code, 0)
			c.fileWriter.Wrq <- wrq
		case data.Unmarshal(buf[:n]) == nil:
			c.ack(conn, addr, OpData, data.Block)
			//log.Printf("received block: %v\n", data.Block)
			// assign memory for every data pkt, maybe optimized using pooled buffer
			buf = make([]byte, DatagramSize)
			c.fileWriter.FileData <- data
			if n < DatagramSize {
				c.fileWriter.FileId <- data.FileId
				log.Printf("file id: [%d] received", data.FileId)
			}
		}
	}
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
