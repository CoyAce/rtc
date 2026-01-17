package core

import (
	"bytes"
	"errors"
	"log"
	"net"
	"slices"
	"sync"
	"syscall"
	"time"
)

type Server struct {
	SignMap       map[string]Sign
	Retries       uint8         // the number of times to retry a failed transmission
	Timeout       time.Duration // the duration to wait for an acknowledgement
	fileMap       map[uint32]WriteReq
	audioMap      map[uint32]WriteReq
	audioReceiver map[uint32][]string
	signLock      sync.Mutex
	wrqLock       sync.Mutex
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
			s.signLock.Lock()
			s.SignMap[addr.String()] = sign
			s.signLock.Unlock()
			s.ack(conn, addr, OpSign, 0)
			log.Printf("[%s] set sign: [%s]", addr.String(), sign)
		case msg.Unmarshal(pkt) == nil:
			go s.handle(msg.Sign, pkt)
			log.Printf("received msg [%s] from [%s]", string(msg.Payload), addr.String())
			s.ack(conn, addr, OpSignedMSG, 0)
		case wrq.Unmarshal(pkt) == nil:
			go s.handle(s.findSignByUUID(wrq.UUID), pkt)
			s.wrqLock.Lock()
			switch wrq.Code {
			case OpAudioCall:
				s.addAudioStream(wrq)
				fallthrough
			case OpAcceptAudioCall:
				s.addAudioReceiver(wrq)
			case OpEndAudioCall:
				s.deleteAudioReceiver(wrq)
				s.cleanupAudioResource(wrq)
			default:
				s.addFile(wrq)
			}
			s.wrqLock.Unlock()
			s.ack(conn, addr, wrq.Code, 0)
		case data.Unmarshal(pkt) == nil:
			isAudioCall := s.audioMap[data.FileId].FileId == data.FileId
			if isAudioCall {
				s.handleStreamData(conn, data, pkt, addr)
				continue
			}
			s.handleFileData(conn, data, pkt, addr)
		}
	}
}

func (s *Server) handleStreamData(conn net.PacketConn, data Data, pkt []byte, addr net.Addr) {
	go s.handleStream(data.FileId, addr, pkt)
	s.ack(conn, addr, OpData, data.Block)
}

func (s *Server) handleFileData(conn net.PacketConn, data Data, pkt []byte, addr net.Addr) {
	go s.handle(s.findSignByFileId(data.FileId), pkt)
	s.ack(conn, addr, OpData, data.Block)
	b := data.Payload.(*bytes.Buffer)
	if b.Len() < BlockSize {
		s.wrqLock.Lock()
		delete(s.fileMap, data.FileId)
		s.wrqLock.Unlock()
	}
}

func (s *Server) cleanupAudioResource(wrq WriteReq) {
	if len(s.audioReceiver[wrq.FileId]) == 0 {
		delete(s.audioReceiver, wrq.FileId)
		delete(s.audioMap, wrq.FileId)
	}
}

func (s *Server) deleteAudioReceiver(wrq WriteReq) {
	s.audioReceiver[wrq.FileId] = slices.DeleteFunc(s.audioReceiver[wrq.FileId], func(e string) bool {
		return e == wrq.UUID
	})
}

func (s *Server) addFile(wrq WriteReq) {
	s.fileMap[wrq.FileId] = wrq
}

func (s *Server) addAudioStream(wrq WriteReq) {
	s.audioMap[wrq.FileId] = wrq
}

func (s *Server) addAudioReceiver(wrq WriteReq) {
	if !slices.Contains(s.audioReceiver[wrq.FileId], wrq.UUID) {
		s.audioReceiver[wrq.FileId] = append(s.audioReceiver[wrq.FileId], wrq.UUID)
	}
}

func (s *Server) init() {
	s.SignMap = make(map[string]Sign)
	s.fileMap = make(map[uint32]WriteReq)
	s.audioMap = make(map[uint32]WriteReq)
	s.audioReceiver = make(map[uint32][]string)

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

func (s *Server) findAddrByUUID(uuid string) string {
	for addr, v := range s.SignMap {
		if v.UUID == uuid {
			return addr
		}
	}
	return ""
}

func (s *Server) findSignByFileId(fileId uint32) Sign {
	s.wrqLock.Lock()
	wrq := s.fileMap[fileId]
	s.wrqLock.Unlock()
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

func (s *Server) handleStream(fileId uint32, sender net.Addr, bytes []byte) {
	senderSign := s.SignMap[sender.String()]
	receivers := s.audioReceiver[fileId]
	for _, UUID := range receivers {
		if UUID != senderSign.UUID {
			receiverAddr := s.findAddrByUUID(UUID)
			go s.connectAndDispatch(receiverAddr, bytes)
		}
	}
}

func (s *Server) handle(sign Sign, bytes []byte) {
	s.signLock.Lock()
	defer s.signLock.Unlock()
	for addr, v := range s.SignMap {
		if v.Sign == sign.Sign && v.UUID != sign.UUID {
			// use goroutine to avoid blocking by slow connection
			go s.connectAndDispatch(addr, bytes)
		}
	}
}

func (s *Server) connectAndDispatch(addr string, bytes []byte) {
	conn, err := net.Dial("udp", addr)
	if err != nil {
		log.Printf("[%s] dial failed: %v", addr, err)
		s.signLock.Lock()
		delete(s.SignMap, addr)
		s.signLock.Unlock()
	}
	defer func() { _ = conn.Close() }()

	ad, _ := net.ResolveUDPAddr("udp", addr)
	s.dispatch(ad, conn, bytes)
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
				s.signLock.Lock()
				delete(s.SignMap, clientAddr.String())
				s.signLock.Unlock()
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
