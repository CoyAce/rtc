package core

import (
	"bytes"
	"errors"
	"log"
	"net"
	"sync"
	"syscall"
	"time"
)

type Server struct {
	SignMap map[string]Sign
	WrqMap  map[uint32]WriteReq
	Retries uint8         // the number of times to retry a failed transmission
	Timeout time.Duration // the duration to wait for an acknowledgement
	lock    sync.Mutex
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

	s.init()

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

		pkt := buf[:n]
		switch {
		case sign.Unmarshal(pkt) == nil:
			s.SignMap[addr.String()] = sign
			s.ack(conn, addr, OpSign, 0)
			log.Printf("[%s] set sign: [%s]", addr.String(), sign)
		case msg.Unmarshal(pkt) == nil:
			go s.handle(msg.Sign, pkt)
			log.Printf("received msg [%s] from [%s]", string(msg.Payload), addr.String())
			s.ack(conn, addr, OpSignedMSG, 0)
		case wrq.Unmarshal(pkt) == nil:
			if wrq.Code == OpSyncIcon {
				go s.handle(s.findSignByUUID(wrq.UUID), pkt)
				s.WrqMap[wrq.FileId] = wrq
				s.ack(conn, addr, OpSyncIcon, 0)
			}
		case data.Unmarshal(pkt) == nil:
			go s.handle(s.findSignByFileId(data.FileId), pkt)
			s.ack(conn, addr, OpData, data.Block)
			b := data.Payload.(*bytes.Buffer)
			if b.Len() < BlockSize {
				delete(s.WrqMap, wrq.FileId)
			}
		}
	}
}

func (s *Server) init() {
	s.SignMap = make(map[string]Sign)
	s.WrqMap = make(map[uint32]WriteReq)

	if s.Retries == 0 {
		s.Retries = 3
	}

	if s.Timeout == 0 {
		s.Timeout = 6 * time.Second
	}
}

func (s *Server) findSignByUUID(uuid string) Sign {
	for _, sign := range s.SignMap {
		if sign.UUID == uuid {
			return sign
		}
	}
	return Sign{UUID: uuid}
}

func (s *Server) findSignByFileId(fileId uint32) Sign {
	wrq := s.WrqMap[fileId]
	return s.findSignByUUID(wrq.UUID)
}

func (s *Server) ack(conn net.PacketConn, clientAddr net.Addr, code OpCode, block uint32) {
	ack := Ack{SrcOp: code, Block: block}
	pkt, err := ack.Marshal()
	_, err = conn.WriteTo(pkt, clientAddr)
	if err != nil {
		log.Printf("[%s] write failed: %v", clientAddr, err)
		return
	}
}

func (s *Server) handle(sign Sign, bytes []byte) {
	for addr, v := range s.SignMap {
		if v.Sign == sign.Sign && v.UUID != sign.UUID {
			conn, err := net.Dial("udp", addr)
			if err != nil {
				log.Printf("[%s] dial failed: %v", addr, err)
				s.lock.Lock()
				delete(s.SignMap, addr)
				s.lock.Unlock()
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
				s.lock.Lock()
				delete(s.SignMap, clientAddr.String())
				s.lock.Unlock()
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
