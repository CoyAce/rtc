package rtc

import (
	"bytes"
	"encoding/binary"
	"errors"
)

const (
	DatagramSize = 1024
	BlockSize    = DatagramSize - 4 // the DatagramSize minus a 4-byte header
)

type OpCode uint16

const (
	OpRRQ OpCode = iota + 1
	OpWRQ
	OpData
	OpSimpleMSG
	OpAck
	OpErr
)

type SimpleMessage struct {
	Payload []byte
}

func (m *SimpleMessage) Marshal() ([]byte, error) {
	size := len(m.Payload)
	if size+2 > BlockSize {
		return nil, errors.New("packet is greater than BlockSize")
	}
	b := new(bytes.Buffer)
	b.Grow(size + 2)

	err := binary.Write(b, binary.BigEndian, OpSimpleMSG) // write operation code
	if err != nil {
		return nil, err
	}

	b.Write(m.Payload)
	return b.Bytes(), nil
}

func (m *SimpleMessage) Unmarshal(p []byte) error {
	if l := len(p); l < 4 || l > DatagramSize {
		return errors.New("invalid DATA")
	}
	var opcode OpCode
	err := binary.Read(bytes.NewReader(p[:2]), binary.BigEndian, &opcode)
	if err != nil || opcode != OpSimpleMSG {
		return errors.New("invalid DATA")
	}

	m.Payload = p[2:]
	return nil
}

type Ack uint32

func (a *Ack) Marshal() ([]byte, error) {
	cap := 2 + 4 // operation code + block number

	b := new(bytes.Buffer)
	b.Grow(cap)

	err := binary.Write(b, binary.BigEndian, uint16(OpAck)) // write operation code
	if err != nil {
		return nil, err
	}

	err = binary.Write(b, binary.BigEndian, a) // write block number
	if err != nil {
		return nil, err
	}

	return b.Bytes(), nil
}

func (a *Ack) Unmarshal(p []byte) error {
	var code OpCode
	r := bytes.NewReader(p)

	err := binary.Read(r, binary.BigEndian, &code) // read operation code
	if err != nil {
		return err
	}

	if code != OpAck {
		return errors.New("invalid DATA")
	}

	return binary.Read(r, binary.BigEndian, a) // read block number
}

type ErrCode uint16

const (
	ErrUnknown ErrCode = iota
	ErrIllegalOp
)
