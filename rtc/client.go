package rtc

import (
	"log"
	"net"
	"time"
)

type Client struct {
	Conn       net.Conn
	Sign       Sign
	ServerAddr string
	Retries    uint8         // the number of times to retry a failed transmission
	Timeout    time.Duration // the duration to wait for an acknowledgement
}

func (c *Client) ChangeSign(sign string) {
	c.Sign = Sign(sign)
	bytes, err := c.Sign.Marshal()
	if err != nil {
		log.Printf("[%s] marshal failed: %v", sign, err)
	}
	if c.Conn == nil {
		conn, err := net.Dial("udp", c.ServerAddr)
		if err != nil {
			log.Printf("[%s] dial failed: %v", c.ServerAddr, err)
		}
		c.Conn = conn
	}
	c.sendPacket(c.Conn, bytes)
}

func (c *Client) SendText(text string) {
	conn, err := net.Dial("udp", c.ServerAddr)
	if err != nil {
		log.Printf("[%s] dial failed: %v", c.ServerAddr, err)
	}
	defer func() { _ = conn.Close() }()
	msg := SignedMessage{Sign: string(c.Sign), Payload: []byte(text)}
	bytes, err := msg.Marshal()
	if err != nil {
		log.Printf("[%s] marshal failed: %v", text, err)
	}

	c.sendPacket(conn, bytes)
}

func (c *Client) ReceiveText() {
	var msg SignedMessage
	buf := make([]byte, DatagramSize)
	_ = c.Conn.SetReadDeadline(time.Now().Add(c.Timeout))
	n, err := c.Conn.Read(buf)
	if err != nil {
		if nErr, ok := err.(net.Error); ok && nErr.Timeout() {
			// log.Printf("receive text timeout")
		}
		//log.Printf("[%s] receive text: %v", c.ServerAddr, err)
	}
	if msg.Unmarshal(buf[:n]) == nil {
		s := string(msg.Payload)
		log.Printf("received text [%s]\n", s)
	}
}

func (c *Client) sendPacket(conn net.Conn, bytes []byte) {
	var ackPkt Ack
	buf := make([]byte, DatagramSize)
RETRY:
	for i := c.Retries; i > 0; i-- {
		_, err := conn.Write(bytes)
		if err != nil {
			log.Printf("[%s] write failed: %v", c.ServerAddr, err)
			return
		}

		// wait for the Server's ACK packet
		_ = conn.SetReadDeadline(time.Now().Add(c.Timeout))
		_, err = conn.Read(buf)

		if err != nil {
			if nErr, ok := err.(net.Error); ok && nErr.Timeout() {
				log.Printf("waiting for ACK timeout")
				continue RETRY
			}
			log.Printf("[%s] waiting for ACK: %v", c.ServerAddr, err)
		}

		switch {
		case ackPkt.Unmarshal(buf) == nil:
			return
		default:
			log.Printf("[%s] bad packet", c.ServerAddr)
		}
	}
	log.Printf("[%s] exhausted retries", c.ServerAddr)
}
