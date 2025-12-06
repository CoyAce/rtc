package rtc

import (
	"errors"
	"log"
	"net"
	"time"
)

type Server struct {
	SignMap map[net.Addr]Sign
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

	s.SignMap = make(map[net.Addr]Sign)

	if s.Retries == 0 {
		s.Retries = 10
	}

	if s.Timeout == 0 {
		s.Timeout = 6 * time.Second
	}

	var (
		sign Sign
		msg  SignedMessage
	)

	for {
		buf := make([]byte, DatagramSize)
		n, addr, err := conn.ReadFrom(buf)
		if err != nil {
			return err
		}

		bytes := buf[:n]
		switch {
		case sign.Unmarshal(bytes) == nil:
			s.SignMap[addr] = sign
			log.Printf("[%s] set sign: [%s]", addr.String(), sign)
			go s.ack(conn, addr)
		case msg.Unmarshal(bytes) == nil:
			log.Printf("received msg [%s] from [%s]", string(msg.Payload), addr.String())
			go s.handle(conn, msg.Sign, bytes)
			go s.ack(conn, addr)
		}
	}
}

func (s *Server) ack(conn net.PacketConn, clientAddr net.Addr) {
	ack := Ack(0)
	bytes, err := ack.Marshal()
	_, err = conn.WriteTo(bytes, clientAddr)
	if err != nil {
		log.Printf("[%s] write failed: %v", clientAddr, err)
		return
	}
	// log.Printf("[%s] write ack finished, soucre addr [%s]", clientAddr, conn.LocalAddr())
}

func (s *Server) handle(conn net.PacketConn, sign string, bytes []byte) {
	for k, v := range s.SignMap {
		if v == Sign(sign) {
			s.dispatch(conn, k, bytes)
		}
	}
}

func (s *Server) dispatch(conn net.PacketConn, clientAddr net.Addr, bytes []byte) {
	_, err := conn.WriteTo(bytes, clientAddr)
	if err != nil {
		log.Printf("[%s] write failed: %v", clientAddr, err)
		return
	}
}
