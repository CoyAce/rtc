package core

import (
	"bytes"
	"net"
	"testing"
)

func TestListenPacketUDP(t *testing.T) {
	// init data
	signAck := Ack{SrcOp: OpSign, Block: 0}
	signAckBytes, err := signAck.Marshal()
	msgAck := Ack{SrcOp: OpSignedMSG, Block: 0}
	msgAckBytes, err := msgAck.Marshal()

	sign := Sign("test")
	signBytes, _ := sign.Marshal()

	uuid := "#00001"
	text := "beautiful world"
	msg := SignedMessage{Sign: string(sign), UUID: uuid, Payload: []byte(text)}
	msgBytes, err := msg.Marshal()

	serverAddr := setUpServer(t)
	client, err := setUpClient(t)
	defer func() { _ = client.Close() }()

	// test send sign, server should ack
	buf := make([]byte, DatagramSize)
	sAddr, _ := net.ResolveUDPAddr("udp", serverAddr)

	// send sign
	_, err = client.WriteTo(signBytes, sAddr)
	if err != nil {
		t.Fatal(err)
	}

	// read ack
	n, _, err := client.ReadFrom(buf)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(signAckBytes, buf[:n]) {
		t.Errorf("expected reply %q; actual reply %q", signAckBytes, buf[:n])
	}

	// send text
	clientA := Client{ServerAddr: serverAddr, Status: make(chan struct{}), UUID: uuid, Sign: sign}
	go func() {
		clientA.ListenAndServe("127.0.0.1:")
	}()
	clientA.Ready()
	clientA.SendText(text)

	// read text
	n, _, err = client.ReadFrom(buf)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(msgBytes, buf[:n]) {
		t.Errorf("expected reply %q; actual reply %q", signAckBytes, buf[:n])
	}
	// send ack
	client.WriteTo(msgAckBytes, sAddr)

	// test send text, server should ack
	_, err = client.WriteTo(msgBytes, sAddr)
	if err != nil {
		t.Fatal(err)
	}
	// read ack
	n, _, err = client.ReadFrom(buf)
	if !bytes.Equal(msgAckBytes, buf[:n]) {
		t.Errorf("expected reply %q; actual reply %q", signAckBytes, buf[:n])
	}
}

func setUpClient(t *testing.T) (net.PacketConn, error) {
	client, err := net.ListenPacket("udp", "127.0.0.1:")
	if err != nil {
		t.Fatal(err)
	}
	return client, err
}

func setUpServer(t *testing.T) string {
	s := Server{}
	serverAddr := "127.0.0.1:52000"
	go func() {
		t.Error(s.ListenAndServe(serverAddr))
	}()
	return serverAddr
}
