package core

import (
	"log"
	"net"
	"time"
)

type Client struct {
	UUID       string
	Status     chan struct{}
	Conn       net.PacketConn
	Sign       Sign
	ServerAddr string
	SAddr      net.Addr
	Retries    uint8         // the number of times to retry a failed transmission
	Timeout    time.Duration // the duration to wait for an acknowledgement
}

func (c *Client) Ready() {
	if c.Status != nil {
		<-c.Status
	}
}

func (c *Client) SendText(text string) {
	conn, err := net.Dial("udp", c.ServerAddr)
	if err != nil {
		log.Printf("[%s] dial failed: %v", c.ServerAddr, err)
	}
	defer func() { _ = conn.Close() }()

	msg := SignedMessage{Sign: string(c.Sign), UUID: c.UUID, Payload: []byte(text)}
	bytes, err := msg.Marshal()
	if err != nil {
		log.Printf("[%s] marshal failed: %v", text, err)
	}

	c.sendPacket(conn, bytes)
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

	c.SAddr, err = net.ResolveUDPAddr("udp", c.ServerAddr)
	c.sendSign()

	log.Printf("Listening on %s ...\n", conn.LocalAddr())
	c.serve(conn)
}

func (c *Client) sendSign() {
	bytes, err := c.Sign.Marshal()
	if err != nil {
		log.Printf("[%s] marshal failed: %v", c.Sign, err)
	}

	c.sendPacketWithPacketConn(bytes)
}

func (c *Client) serve(conn net.PacketConn) {
	var msg SignedMessage
	buf := make([]byte, DatagramSize)

	for {
		_ = conn.SetReadDeadline(time.Now().Add(c.Timeout))
		n, addr, err := conn.ReadFrom(buf)
		if err != nil {
			if nErr, ok := err.(net.Error); ok && nErr.Timeout() {
				//log.Printf("receive text timeout")
			}
			//log.Printf("[%s] receive text: %v", c.ServerAddr, err)
			continue
		}

		if msg.Unmarshal(buf[:n]) == nil {
			s := string(msg.Payload)
			log.Printf("received text [%s] from [%s]\n", s, msg.UUID)
			c.ack(conn, addr)
		}
	}
}

func (c *Client) ack(conn net.PacketConn, clientAddr net.Addr) {
	ack := Ack(0)
	bytes, err := ack.Marshal()
	_, err = conn.WriteTo(bytes, clientAddr)
	if err != nil {
		log.Printf("[%s] write failed: %v", clientAddr, err)
		return
	}
	// log.Printf("[%s] write ack finished, soucre addr [%s]", clientAddr, conn.LocalAddr())
}

func (c *Client) sendPacketWithPacketConn(bytes []byte) {
	var ackPkt Ack
	buf := make([]byte, DatagramSize)
RETRY:
	for i := c.Retries; i > 0; i-- {
		_, err := c.Conn.WriteTo(bytes, c.SAddr)
		if err != nil {
			log.Printf("[%s] write failed: %v", c.ServerAddr, err)
			return
		}

		// wait for the Server's ACK packet
		_ = c.Conn.SetReadDeadline(time.Now().Add(c.Timeout))
		_, addr, err := c.Conn.ReadFrom(buf)

		if err != nil {
			if nErr, ok := err.(net.Error); ok && nErr.Timeout() {
				log.Printf("waiting for ACK timeout")
				continue RETRY
			}
			log.Printf("[%s] waiting for ACK: %v", c.ServerAddr, err)
		}

		if addr.String() != c.ServerAddr {
			log.Fatalf("received reply from %q instead of %q", addr, c.ServerAddr)
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
