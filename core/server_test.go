package core

import (
	"bytes"
	"net"
	"testing"
)

func TestListenPacketUDP(t *testing.T) {
	s := Server{}
	serverAddr := "127.0.0.1:52000"
	go func() {
		t.Error(s.ListenAndServe(serverAddr))
	}()

	client, err := net.ListenPacket("udp", "127.0.0.1:")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = client.Close() }()

	// test send sign, server should ack
	buf := make([]byte, DatagramSize)
	testSign := Sign("test")
	signBytes, _ := testSign.Marshal()
	sAddr, _ := net.ResolveUDPAddr("udp", serverAddr)
	_, err = client.WriteTo(signBytes, sAddr)
	if err != nil {
		t.Fatal(err)
	}

	// read ack
	n, _, err := client.ReadFrom(buf)
	if err != nil {
		t.Fatal(err)
	}
	ack := Ack(0)
	ackBytes, err := ack.Marshal()
	if !bytes.Equal(ackBytes, buf[:n]) {
		t.Errorf("expected reply %q; actual reply %q", ackBytes, buf[:n])
	}

	// test send text
	clientA := Client{ServerAddr: serverAddr, UUID: "#00001", Sign: testSign}
	go func() {
		clientA.ListenAndServe("127.0.0.1:")
	}()

	text := "beautiful world"
	clientA.SendText(text)
	msg := SignedMessage{Sign: string(testSign), UUID: clientA.UUID, Payload: []byte(text)}
	msgBytes, err := msg.Marshal()
	n, _, err = client.ReadFrom(buf)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(msgBytes, buf[:n]) {
		t.Errorf("expected reply %q; actual reply %q", ackBytes, buf[:n])
	}
	client.WriteTo(ackBytes, sAddr)

	// test send text, server should ack
	_, err = client.WriteTo(msgBytes, sAddr)
	if err != nil {
		t.Fatal(err)
	}
	n, _, err = client.ReadFrom(buf)
	if !bytes.Equal(ackBytes, buf[:n]) {
		t.Errorf("expected reply %q; actual reply %q", ackBytes, buf[:n])
	}
}
