package main

import (
	"aether-go/pkg"
	"bytes"
	"encoding/binary"
	"fmt"
	"math/rand"
	"net"
	"time"
)

func main() {
	addr := net.TCPAddr{
		IP:   net.IPv4(127, 0, 0, 1),
		Port: 30000,
		Zone: "",
	}

	tcp, err := net.DialTCP("tcp4", nil, &addr)
	if err != nil {
		panic("Unable to dial to remote")
	}

	read, write := pkg.NewRequestReplyAetherConnection(*tcp, ServerDeserializer{})

	go processRequestResponseClientReceiver(read)

	for {
		resChan := write.Request(&Ping{
			rPayload: rand.Uint64(),
		})

		go func() {
			res := <-resChan

			switch res.(type) {
			case *Pong:
				pong := res.(*Pong)

				if pong.counter%1_000_000 == 0 {
					fmt.Println("Got", pong.counter, "nth response at", time.Now())
				}
			}
		}()
	}
}

func processRequestResponseClientReceiver(c pkg.RequestReplyReadHandle) {
	for {
		frame := c.Read()

		switch frame.(type) {
		case pkg.NotifyMessageRequestType:
			_ = frame.(pkg.NotifyMessageRequestType).Msg

		case pkg.RequestMessageRequestType:
			_ = frame.(pkg.RequestMessageRequestType).Msg
		}
	}
}

type ServerDeserializer struct {
}

func (s ServerDeserializer) Deserialize(f []byte) pkg.Serializable {
	switch f[0] {
	case 0x00:
		return &Pong{
			counter: binary.LittleEndian.Uint64(f[1:]),
		}
	default:
		panic("Unknown message type")
	}
}

type Pong struct {
	counter uint64
}

func (c *Pong) Serialize(buf *bytes.Buffer) {
	buf.WriteByte(0x00)
	binary.LittleEndian.PutUint64(buf.Bytes(), c.counter)
}

type Ping struct {
	rPayload uint64
}

func (c *Ping) Serialize(buf *bytes.Buffer) {
	buf.WriteByte(0x00)

	buf.WriteByte(byte(c.rPayload))
	buf.WriteByte(byte(c.rPayload >> 8))
	buf.WriteByte(byte(c.rPayload >> 16))
	buf.WriteByte(byte(c.rPayload >> 24))
	buf.WriteByte(byte(c.rPayload >> 32))
	buf.WriteByte(byte(c.rPayload >> 40))
	buf.WriteByte(byte(c.rPayload >> 48))
	buf.WriteByte(byte(c.rPayload >> 54))
	buf.WriteByte(byte(c.rPayload >> 60))
}
