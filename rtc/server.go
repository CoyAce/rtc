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

	var ssmg SimpleMessage

	for {
		buf := make([]byte, DatagramSize)
		_, addr, err := conn.ReadFrom(buf)
		if err != nil {
			return err
		}
		err = ssmg.Unmarshal(buf)
		if err != nil {
			log.Printf("[%s] bad request: %v", addr, err)
			continue
		}

		go s.handle(addr.String(), ssmg)
	}
}

func (s *Server) handle(clientAddr string, ssmg SimpleMessage) {
	conn, err := net.Dial("udp", clientAddr)
	if err != nil {
		log.Printf("[%s] dial failed: %v", clientAddr, err)
		return
	}
	defer func() { _ = conn.Close() }()

}
