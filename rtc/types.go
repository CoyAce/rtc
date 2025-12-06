package rtc

import (
	"bytes"
	"encoding/binary"
	"errors"
	"strings"
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
	OpSign
	OpSignedMSG
	OpAck
	OpErr
)

type Sign string

func (sign *Sign) Marshal() ([]byte, error) {
	b := new(bytes.Buffer)
	b.Grow(len(*sign) + 2)

	err := binary.Write(b, binary.BigEndian, OpSign) // write operation code
	if err != nil {
		return nil, err
	}

	_, err = b.WriteString(string(*sign)) // write Sign
	if err != nil {
		return nil, err
	}

	err = b.WriteByte(0) // write 0 byte
	if err != nil {
		return nil, err
	}

	return b.Bytes(), nil
}

func (sign *Sign) Unmarshal(p []byte) error {
	r := bytes.NewBuffer(p)
	var opcode OpCode
	err := binary.Read(r, binary.BigEndian, &opcode)
	if err != nil || opcode != OpSign {
		return errors.New("invalid DATA")
	}

	s, err := r.ReadString(0) // read sign
	if err != nil {
		return errors.New("invalid DATA")
	}
	s = strings.TrimRight((s), "\x00") // remove the 0-byte
	if len(s) == 0 {
		return errors.New("invalid DATA")
	}
	*sign = Sign(s)
	return nil
}

type SignedMessage struct {
	Sign    string
	Payload []byte
}

func (m *SignedMessage) Marshal() ([]byte, error) {
	size := len(m.Sign) + 1 + len(m.Payload)
	if size+2 > BlockSize {
		return nil, errors.New("packet is greater than BlockSize")
	}
	b := new(bytes.Buffer)
	b.Grow(size + 2)

	err := binary.Write(b, binary.BigEndian, OpSignedMSG) // write operation code
	if err != nil {
		return nil, err
	}

	_, err = b.WriteString(m.Sign) // write Sign
	if err != nil {
		return nil, err
	}

	err = b.WriteByte(0) // write 0 byte
	if err != nil {
		return nil, err
	}

	b.Write(m.Payload)
	return b.Bytes(), nil
}

func (m *SignedMessage) Unmarshal(p []byte) error {
	if l := len(p); l < 4 || l > DatagramSize {
		return errors.New("invalid DATA")
	}
	r := bytes.NewBuffer(p)
	var opcode OpCode
	err := binary.Read(r, binary.BigEndian, &opcode)
	if err != nil || opcode != OpSignedMSG {
		return errors.New("invalid DATA")
	}

	m.Sign, err = r.ReadString(0) // read sign
	if err != nil {
		return errors.New("invalid DATA")
	}
	m.Sign = strings.TrimRight((m.Sign), "\x00") // remove the 0-byte
	if len(m.Sign) == 0 {
		return errors.New("invalid DATA")
	}
	m.Payload = r.Bytes()
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
