package core

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"strings"
)

const (
	DatagramSize = 1286
	BlockSize    = DatagramSize - 2 - 4 // DataGramSize - OpCode -Block
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
	OpSyncIcon
)

func writeString(b *bytes.Buffer, str string) error {
	_, err := b.WriteString(str) // write str
	if err != nil {
		return err
	}

	err = b.WriteByte(0) // write 0 byte
	if err != nil {
		return err
	}
	return nil
}

func readString(r *bytes.Buffer) (string, error) {
	str, err := r.ReadString(0) //read filename
	if err != nil {
		return "", err
	}

	str = strings.TrimRight(str, "\x00") // remove the 0-byte
	if len(str) == 0 {
		return "", err
	}
	return str, nil
}

type WriteReq struct {
	Code     OpCode
	UUID     string
	Filename string
}

func (q *WriteReq) Marshal() ([]byte, error) {
	size := 2 + len(q.UUID) + 1 + len(q.Filename) + 1
	b := new(bytes.Buffer)
	b.Grow(size)

	if q.Code != OpWRQ && q.Code != OpSyncIcon {
		return nil, errors.New("invalid WRQ")
	}

	err := binary.Write(b, binary.BigEndian, q.Code) // write operation code
	if err != nil {
		return nil, err
	}

	err = writeString(b, q.UUID) // write UUID
	if err != nil {
		return nil, err
	}

	err = writeString(b, q.Filename) // write filename
	if err != nil {
		return nil, err
	}

	return b.Bytes(), nil
}

func (q *WriteReq) Unmarshal(p []byte) error {
	r := bytes.NewBuffer(p)

	err := binary.Read(r, binary.BigEndian, &q.Code) // read operation code
	if err != nil {
		return err
	}

	if q.Code != OpWRQ && q.Code != OpSyncIcon {
		return errors.New("invalid WRQ")
	}

	q.UUID, err = readString(r)
	if err != nil {
		return errors.New("invalid WRQ")
	}

	q.Filename, err = readString(r)
	if err != nil {
		return errors.New("invalid WRQ")
	}

	return nil
}

type Data struct {
	Block   uint32
	Payload io.Reader
}

func (d *Data) Marshal() ([]byte, error) {
	b := new(bytes.Buffer)
	b.Grow(DatagramSize)

	d.Block++ // block numbers increment from 1

	err := binary.Write(b, binary.BigEndian, OpData) // write operation code
	if err != nil {
		return nil, err
	}

	err = binary.Write(b, binary.BigEndian, d.Block) // write block number
	if err != nil {
		return nil, err
	}

	// write up to BlockSize worth of bytes
	_, err = io.CopyN(b, d.Payload, BlockSize)
	if err != nil && err != io.EOF {
		return nil, err
	}

	return b.Bytes(), nil
}

func (d *Data) Unmarshal(p []byte) error {
	if l := len(p); l < 6 || l > DatagramSize {
		return errors.New("invalid DATA")
	}

	var code OpCode

	err := binary.Read(bytes.NewReader(p[:2]), binary.BigEndian, &code)
	if err != nil || code != OpData {
		return errors.New("invalid DATA")
	}

	err = binary.Read(bytes.NewReader(p[2:6]), binary.BigEndian, &d.Block)
	if err != nil {
		return errors.New("invalid DATA")
	}

	d.Payload = bytes.NewBuffer(p[6:])

	return nil
}

type Sign string

func (sign *Sign) Marshal() ([]byte, error) {
	b := new(bytes.Buffer)
	b.Grow(len(*sign) + 2)

	err := binary.Write(b, binary.BigEndian, OpSign) // write operation code
	if err != nil {
		return nil, err
	}

	err = writeString(b, string(*sign)) // write Sign
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

	s, err := readString(r) // read sign
	if err != nil {
		return errors.New("invalid DATA")
	}
	*sign = Sign(s)
	return nil
}

type SignedMessage struct {
	Sign    string
	UUID    string
	Payload []byte
}

func (m *SignedMessage) Marshal() ([]byte, error) {
	size := 2 + len(m.Sign) + 1 + len(m.UUID) + 1 + len(m.Payload)
	if size > DatagramSize {
		return nil, errors.New("packet is greater than DatagramSize")
	}
	b := new(bytes.Buffer)
	b.Grow(size)

	err := binary.Write(b, binary.BigEndian, OpSignedMSG) // write operation code
	if err != nil {
		return nil, err
	}

	err = writeString(b, m.Sign) // write Sign
	if err != nil {
		return nil, err
	}

	err = writeString(b, m.UUID) // write UUID
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

	m.Sign, err = readString(r) // read sign
	if err != nil {
		return errors.New("invalid DATA")
	}

	m.UUID, err = readString(r) // read sign
	if err != nil {
		return errors.New("invalid DATA")
	}

	m.Payload = r.Bytes()
	return nil
}

type Ack struct {
	SrcOp OpCode
	Block uint32
}

func (a *Ack) Marshal() ([]byte, error) {
	cap := 2 + 2 + 4 // operation code + source operation code + block number

	b := new(bytes.Buffer)
	b.Grow(cap)

	err := binary.Write(b, binary.BigEndian, uint16(OpAck)) // write operation code
	if err != nil {
		return nil, err
	}

	err = binary.Write(b, binary.BigEndian, uint16(a.SrcOp)) // write source operation code
	if err != nil {
		return nil, err
	}

	err = binary.Write(b, binary.BigEndian, a.Block) // write block number
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

	err = binary.Read(r, binary.BigEndian, &a.SrcOp) // read source operation code

	return binary.Read(r, binary.BigEndian, &a.Block) // read block number
}

type ErrCode uint16

const (
	ErrUnknown ErrCode = iota
	ErrIllegalOp
)
