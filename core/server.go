package core

import (
	"errors"
	"log"
	"net"
	"syscall"
	"time"
)

type Server struct {
	SignMap map[string]Sign
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

	s.SignMap = make(map[string]Sign)

	if s.Retries == 0 {
		s.Retries = 3
	}

	if s.Timeout == 0 {
		s.Timeout = 6 * time.Second
	}

	var (
		sign Sign
		msg  SignedMessage
		wrq  WriteReq
		data = Data{}
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
			s.SignMap[addr.String()] = sign
			s.ack(conn, addr, OpSign, 0)
			log.Printf("[%s] set sign: [%s]", addr.String(), sign)
		case msg.Unmarshal(bytes) == nil:
			log.Printf("received msg [%s] from [%s]", string(msg.Payload), addr.String())
			s.ack(conn, addr, OpSignedMSG, 0)
			go s.handle(msg.Sign, bytes)
		case wrq.Unmarshal(bytes) == nil:
			if wrq.Code == OpSyncIcon {
				s.ack(conn, addr, OpSyncIcon, 0)
				go s.handle(msg.Sign, bytes)
			}
		case data.Unmarshal(bytes) == nil:
			s.ack(conn, addr, OpData, data.Block)
			go s.handle(msg.Sign, bytes)
		}
	}
}

func (s *Server) ack(conn net.PacketConn, clientAddr net.Addr, code OpCode, block uint32) {
	ack := Ack{SrcOp: code, Block: block}
	bytes, err := ack.Marshal()
	_, err = conn.WriteTo(bytes, clientAddr)
	if err != nil {
		log.Printf("[%s] write failed: %v", clientAddr, err)
		return
	}
	// log.Printf("[%s] write ack finished, soucre addr [%s]", clientAddr, conn.LocalAddr())
}

func (s *Server) handle(sign string, bytes []byte) {
	for addr, v := range s.SignMap {
		if v == Sign(sign) {
			conn, err := net.Dial("udp", addr)
			if err != nil {
				log.Printf("[%s] dial failed: %v", addr, err)
				delete(s.SignMap, addr)
				continue
			}
			defer func() { _ = conn.Close() }()

			ad, _ := net.ResolveUDPAddr("udp", addr)
			s.dispatch(ad, conn, bytes)
		}
	}
}

func (s *Server) dispatch(clientAddr net.Addr, conn net.Conn, bytes []byte) {
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
			var nErr net.Error
			if errors.As(err, &nErr) && nErr.Timeout() {
				continue RETRY
			}
			if errors.Is(err, syscall.ECONNREFUSED) {
				delete(s.SignMap, clientAddr.String())
				log.Printf("[%s] connection refused", clientAddr)
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
