package rtc

import (
	"errors"
	"log"
	"net"
	"time"
)

type Server struct {
	Retries uint8         // the number of times to retry a failed transmission
	Timeout time.Duration // the duration to wait for an acknowledgement
}

func (s *Server) ListenAndServe(addr string) error {
	conn, err := net.ListenPacket("udp", addr)
	if err != nil {
		return err
	}
	defer func() { _ = conn.Close() }()

	log.Printf("Listening on %s ...\n", conn.LocalAddr())
	return s.Serve(conn)
}

func (s *Server) Serve(conn net.PacketConn) error {
	if conn == nil {
		return errors.New("nil connection")
	}

	if s.Retries == 0 {
		s.Retries = 10
	}

	if s.Timeout == 0 {
		s.Timeout = 6 * time.Second
	}

	var msg SignedMessage

	for {
		buf := make([]byte, DatagramSize)
		dup := make([]byte, DatagramSize)
		_, addr, err := conn.ReadFrom(buf)
		if err != nil {
			return err
		}
		copy(dup, buf)
		err = msg.Unmarshal(buf)
		if err != nil {
			log.Printf("[%s] bad request: %v", addr, err)
			continue
		}

		go s.handle(addr.String(), msg.Sign, dup)
	}
}

func (s *Server) handle(clientAddr string, sign string, bytes []byte) {
	conn, err := net.Dial("udp", clientAddr)
	if err != nil {
		log.Printf("[%s] dial failed: %v", clientAddr, err)
		return
	}
	defer func() { _ = conn.Close() }()

	s.dispatch(clientAddr, conn, bytes)
}

func (s *Server) dispatch(clientAddr string, conn net.Conn, bytes []byte) {
	var (
		ackPkt Ack
	)
	buf := make([]byte, DatagramSize)
RETRY:
	for i := s.Retries; i > 0; i-- {
		_, err := conn.Write(bytes)
		if err != nil {
			log.Printf("[%s] write failed: %v", clientAddr, err)
			return
		}

		// wait for the client's ACK packet
		_ = conn.SetReadDeadline(time.Now().Add(s.Timeout))
		_, err = conn.Read(buf)

		if err != nil {
			if nErr, ok := err.(net.Error); ok && nErr.Timeout() {
				continue RETRY
			}
			log.Printf("[%s] waiting for ACK: %v", clientAddr, err)
			return
		}

		switch {
		case ackPkt.Unmarshal(buf) == nil:
			return
		default:
			log.Printf("[%s] bad packet", clientAddr)
		}
	}
	log.Printf("[%s] exhausted retries", clientAddr)
	return
}
