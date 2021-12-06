package main

import (
	"aether-go/pkg"
	"bytes"
	"encoding/binary"
	"fmt"
	"net"
	"os"
	"time"
)

func main() {
	addr := net.TCPAddr{
		IP:   net.IPv4(127, 0, 0, 1),
		Port: 30000,
		Zone: "",
	}

	l, err := net.ListenTCP("tcp4", &addr)

	if err != nil {
		fmt.Println("Error listening:", err.Error())
		os.Exit(1)
	}

	defer l.Close()

	fmt.Println("Listening on", addr.String())

	for {
		conn, err := l.AcceptTCP()

		readHandle, writeHandle := pkg.NewRequestReplyAetherConnection(*conn, &ClientDeserializer{})

		if err != nil {
			fmt.Println("Error accepting: ", err.Error())
			os.Exit(1)
		}

		go processRequestReplyReceiver(readHandle)
		go processRequestReplySender(writeHandle)
	}
}

func processRequestReplySender(write pkg.RequestReplyWriteHandle) {
	for {
		time.Sleep(1 * time.Second)
	}
}

func processRequestReplyReceiver(c pkg.RequestReplyReadHandle) {
	var counter uint64

	for {
		frame := c.Read()

		switch frame.(type) {
		case pkg.NotifyMessageRequestType:
			_ = frame.(pkg.NotifyMessageRequestType).Msg

		case pkg.RequestMessageRequestType:
			msg := frame.(pkg.RequestMessageRequestType).Msg
			c := frame.(pkg.RequestMessageRequestType).ResponseChan

			switch msg.(type) {
			case *Ping:
				c <- &Pong{
					counter: counter,
				}
			}

			counter = counter + 1
		}
	}
}

type Pong struct {
	counter uint64
}

func (c *Pong) Serialize(buf *bytes.Buffer) {
	buf.WriteByte(0x00)
	buf.WriteByte(byte(c.counter))
	buf.WriteByte(byte(c.counter >> 8))
	buf.WriteByte(byte(c.counter >> 16))
	buf.WriteByte(byte(c.counter >> 24))
	buf.WriteByte(byte(c.counter >> 32))
	buf.WriteByte(byte(c.counter >> 40))
	buf.WriteByte(byte(c.counter >> 48))
	buf.WriteByte(byte(c.counter >> 54))
	buf.WriteByte(byte(c.counter >> 60))
}

type Ping struct {
	rPayload uint64
}

func (c *Ping) Serialize(buf *bytes.Buffer) {
	buf.WriteByte(0x00)
	binary.LittleEndian.PutUint64(buf.Bytes(), c.rPayload)
}

type ClientDeserializer struct {
}

func (s *ClientDeserializer) Deserialize(f []byte) pkg.Serializable {
	switch f[0] {
	case 0x00:
		return &Ping{
			rPayload: binary.LittleEndian.Uint64(f[1:]),
		}
	default:
		panic("Unknown message type")
	}
}
