package core

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
)

const (
	DatagramSize = 1460
	BlockSize    = DatagramSize - 2 - 4 - 4 // DataGramSize - OpCode - FileId - Block
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
	OpSendImage
	OpSendGif
	OpSendVoice
	OpAudioCall
	OpAcceptAudioCall
	OpEndAudioCall
)

var wrqSet = map[OpCode]bool{
	OpWRQ:             true,
	OpSyncIcon:        true,
	OpSendImage:       true,
	OpSendGif:         true,
	OpSendVoice:       true,
	OpAudioCall:       true,
	OpAcceptAudioCall: true,
	OpEndAudioCall:    true,
}

type WriteReq struct {
	Code     OpCode
	FileId   uint32
	UUID     string
	Filename string
	Size     uint64
	Duration uint64
}

func (q *WriteReq) Marshal() ([]byte, error) {
	size := 2 + 4 + len(q.UUID) + 1 + len(q.Filename) + 1 + 8 + 8
	b := new(bytes.Buffer)
	b.Grow(size)

	if !wrqSet[q.Code] {
		return nil, errors.New("invalid WRQ")
	}

	err := binary.Write(b, binary.BigEndian, q.Code) // write operation code
	if err != nil {
		return nil, err
	}

	err = binary.Write(b, binary.BigEndian, q.FileId) // write file id
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

	err = binary.Write(b, binary.BigEndian, q.Size) // write size
	if err != nil {
		return nil, err
	}

	err = binary.Write(b, binary.BigEndian, q.Duration) // write duration
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

	if !wrqSet[q.Code] {
		return errors.New("invalid WRQ")
	}

	err = binary.Read(r, binary.BigEndian, &q.FileId) // read file id
	if err != nil {
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

	err = binary.Read(r, binary.BigEndian, &q.Size) // read size
	if err != nil {
		return errors.New("invalid WRQ")
	}

	err = binary.Read(r, binary.BigEndian, &q.Duration) // read duration
	if err != nil {
		return errors.New("invalid WRQ")
	}

	return nil
}

type Data struct {
	FileId  uint32
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

	err = binary.Write(b, binary.BigEndian, d.FileId) // write file id
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
	if l := len(p); l < 10 || l > DatagramSize {
		return errors.New("invalid DATA")
	}

	var code OpCode

	err := binary.Read(bytes.NewReader(p[:2]), binary.BigEndian, &code) // read operation code
	if err != nil || code != OpData {
		return errors.New("invalid DATA")
	}

	err = binary.Read(bytes.NewReader(p[2:6]), binary.BigEndian, &d.FileId) // read file id
	if err != nil {
		return errors.New("invalid DATA")
	}

	err = binary.Read(bytes.NewReader(p[6:10]), binary.BigEndian, &d.Block) // read block number

	d.Payload = bytes.NewBuffer(p[10:])

	return nil
}

type Sign struct {
	Sign string
	UUID string
}

func (sign *Sign) Marshal() ([]byte, error) {
	b := new(bytes.Buffer)
	b.Grow(2 + len(sign.Sign) + 1 + len(sign.UUID) + 1)

	err := binary.Write(b, binary.BigEndian, OpSign) // write operation code
	if err != nil {
		return nil, err
	}

	err = writeString(b, sign.Sign) // write Sign
	if err != nil {
		return nil, err
	}

	err = writeString(b, sign.UUID) // write UUID
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

	sign.Sign, err = readString(r) // read sign
	if err != nil {
		return errors.New("invalid DATA")
	}

	sign.UUID, err = readString(r) // read UUID
	if err != nil {
		return errors.New("invalid DATA")
	}
	return nil
}

type SignedMessage struct {
	Sign    Sign
	Payload []byte
}

func (m *SignedMessage) Marshal() ([]byte, error) {
	size := 2 + len(m.Sign.Sign) + 1 + len(m.Sign.UUID) + 1 + len(m.Payload)
	if size > DatagramSize {
		return nil, errors.New("packet is greater than DatagramSize")
	}
	b := new(bytes.Buffer)
	b.Grow(size)

	err := binary.Write(b, binary.BigEndian, OpSignedMSG) // write operation code
	if err != nil {
		return nil, err
	}

	sign, err := m.Sign.Marshal()
	if err != nil {
		return nil, err
	}
	b.Write(sign[2:])

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

	m.Sign.Sign, err = readString(r)
	if err != nil {
		return errors.New("invalid DATA")
	}

	m.Sign.UUID, err = readString(r)
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
	size := 2 + 2 + 4 // operation code + source operation code + block number

	b := new(bytes.Buffer)
	b.Grow(size)

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
