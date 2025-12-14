package core

import (
	"encoding/json"
	"errors"
	"log"
	"net"
	"os"
	"syscall"
	"time"
)

type Client struct {
	UUID           string
	Nickname       string
	SignedMessages chan SignedMessage `json:"-"`
	Status         chan struct{}      `json:"-"`
	Conn           net.PacketConn     `json:"-"`
	Connected      bool
	Sign           Sign
	ServerAddr     string
	SAddr          net.Addr      `json:"-"`
	Retries        uint8         // the number of times to retry a failed transmission
	Timeout        time.Duration // the duration to wait for an acknowledgement
}

func (c *Client) Ready() {
	if c.Status != nil {
		<-c.Status
	}
}

func (c *Client) SendText(text string) error {
	conn, err := net.Dial("udp", c.ServerAddr)
	if err != nil {
		log.Printf("[%s] dial failed: %v", c.ServerAddr, err)
	}
	defer func() { _ = conn.Close() }()

	msg := SignedMessage{Sign: string(c.Sign), UUID: c.FullID(), Payload: []byte(text)}
	bytes, err := msg.Marshal()
	if err != nil {
		log.Printf("[%s] marshal failed: %v", text, err)
	}

	return c.sendPacket(conn, bytes)
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
	if c.Status != nil {
		close(c.Status)
	}

	// init
	if c.Retries == 0 {
		c.Retries = 3
	}

	if c.Timeout == 0 {
		c.Timeout = 6 * time.Second
	}

	c.SignedMessages = make(chan SignedMessage)

	c.SAddr, err = net.ResolveUDPAddr("udp", c.ServerAddr)
	go func() {
		// auto reconnect in case of server down
		for {
			c.SendSign()
			time.Sleep(30 * time.Second)
		}
	}()

	log.Printf("Listening on %s ...\n", conn.LocalAddr())
	c.serve(conn)
}

func (c *Client) SendSign() {
	bytes, err := c.Sign.Marshal()
	if err != nil {
		log.Printf("[%s] marshal failed: %v", c.Sign, err)
	}
	_, err = c.Conn.WriteTo(bytes, c.SAddr)
	if err != nil {
		log.Printf("[%s] write failed: %v", c.ServerAddr, err)
		return
	}
}

func (c *Client) serve(conn net.PacketConn) {
	var ackPkt Ack
	var msg SignedMessage
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
		case ackPkt.Unmarshal(buf[:n]) == nil:
			if ackPkt.Op == OpSign {
				c.Connected = true
			}
			continue
		case msg.Unmarshal(buf[:n]) == nil:
			s := string(msg.Payload)
			log.Printf("received text [%s] from [%s]\n", s, msg.UUID)
			c.SignedMessages <- msg
			c.ack(conn, addr, OpSignedMSG)
		}
	}
}

func (c *Client) ack(conn net.PacketConn, clientAddr net.Addr, code OpCode) {
	ack := Ack{Op: code, Block: 0}
	bytes, err := ack.Marshal()
	_, err = conn.WriteTo(bytes, clientAddr)
	if err != nil {
		log.Printf("[%s] write failed: %v", clientAddr, err)
		return
	}
	// log.Printf("[%s] write ack finished, soucre addr [%s]", clientAddr, conn.LocalAddr())
}

func (c *Client) sendPacket(conn net.Conn, bytes []byte) error {
	var ackPkt Ack
	buf := make([]byte, DatagramSize)
RETRY:
	for i := c.Retries; i > 0; i-- {
		_, err := conn.Write(bytes)
		if err != nil {
			log.Printf("[%s] write failed: %v", c.ServerAddr, err)
			return err
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
		case ackPkt.Unmarshal(buf) == nil:
			return nil
		default:
			log.Printf("[%s] bad packet", c.ServerAddr)
		}
	}
	return errors.New("exhausted retries")
}

var configName = "config.json"

func Load() *Client {
	_, err := os.Stat(configName)
	if os.IsNotExist(err) {
		return nil
	}
	file, err := os.Open(configName)
	if err != nil {
		panic(err)
	}
	defer file.Close()
	var c Client
	decoder := json.NewDecoder(file)
	err = decoder.Decode(&c)
	if err != nil {
		panic(err)
	}
	return &c
}

func (c *Client) Store() {
	file, err := os.Create(configName)
	if err != nil {
		panic(err)
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	err = encoder.Encode(&c)
	if err != nil {
		panic(err)
	}
}
