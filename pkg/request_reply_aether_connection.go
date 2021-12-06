package pkg

import (
	"bytes"
	"encoding/binary"
	"math"
	"net"
	"sync"
	"time"
)

type RequestReplyWriteHandle struct {
	callbacks  callbackRingBuf
	msgCounter uint16
	writerChan chan Serializable
}

type RequestReplyReadHandle struct {
	callbacks   callbackRingBuf
	writerChan  chan Serializable
	deserialize Deserializer
	readerChan  chan Serializable
}

type RequestReplyParser struct {
	deserialize Deserializer
}

type NotifyMessageRequestType struct {
	Msg Serializable
}

type RequestMessageRequestType struct {
	Msg          Serializable
	ResponseChan chan Serializable
}

type notifyMessageType struct {
	msg Serializable
}

func (n *notifyMessageType) Serialize(buf *bytes.Buffer) {
	buf.WriteByte(0x00)
	n.msg.Serialize(buf)
}

type requestMessageType struct {
	id  uint16
	msg Serializable
}

func (r *requestMessageType) Serialize(buf *bytes.Buffer) {
	msgId1, msgId2 := getMsgId(r.id)

	buf.WriteByte(0x01)
	buf.WriteByte(msgId1)
	buf.WriteByte(msgId2)
	r.msg.Serialize(buf)
}

type responseMessageType struct {
	id  uint16
	msg Serializable
}

func (r *responseMessageType) Serialize(buf *bytes.Buffer) {
	msgId1, msgId2 := getMsgId(r.id)

	buf.WriteByte(0x02)
	buf.WriteByte(msgId1)
	buf.WriteByte(msgId2)
	r.msg.Serialize(buf)
}

type callbackRingBuf struct {
	buf []chan Serializable
}

func (c *callbackRingBuf) put(id uint16, resChan chan Serializable) {
	c.buf[id] = resChan
}

func (c *callbackRingBuf) get(id uint16) chan Serializable {
	return c.buf[id]
}

func (c *callbackRingBuf) delete(id uint16) {
	c.buf[id] = nil
}

func (c *callbackRingBuf) exists(id uint16) bool {
	return c.buf[id] != nil
}

type callbackContainer struct {
	lock      *sync.Mutex
	callbacks map[uint16]chan Serializable
}

func (c *callbackContainer) put(id uint16, resChan chan Serializable) {
	c.lock.Lock()
	defer c.lock.Unlock()

	c.callbacks[id] = resChan
}

func (c *callbackContainer) get(id uint16) chan Serializable {
	c.lock.Lock()
	defer c.lock.Unlock()

	return c.callbacks[id]
}

func (c *callbackContainer) delete(id uint16) {
	c.lock.Lock()
	defer c.lock.Unlock()

	delete(c.callbacks, id)
}

func (c *callbackContainer) exists(id uint16) bool {
	c.lock.Lock()
	defer c.lock.Unlock()

	if _, ok := c.callbacks[id]; !ok {
		return false
	}

	return true
}

func (r *RequestReplyParser) Deserialize(frame []byte) Serializable {
	switch frame[0] {
	case 0x00:
		return &notifyMessageType{
			msg: r.deserialize.Deserialize(frame[1:]),
		}
	case 0x01:
		return &requestMessageType{
			id:  binary.LittleEndian.Uint16(frame[1:3]),
			msg: r.deserialize.Deserialize(frame[3:]),
		}
	case 0x02:
		return &responseMessageType{
			id:  binary.LittleEndian.Uint16(frame[1:3]),
			msg: r.deserialize.Deserialize(frame[3:]),
		}
	default:
		panic("Unknown message type")
	}
}

func getMsgId(v uint16) (uint8, uint8) {
	return byte(v), byte(v >> 8)
}

func NewRequestReplyAetherConnection(conn net.TCPConn, deserialize Deserializer) (RequestReplyReadHandle, RequestReplyWriteHandle) {
	connection := NewBaseAetherConnection(conn, &RequestReplyParser{
		deserialize,
	})

	read, write := connection.Split()

	callbacks := callbackRingBuf{
		buf: make([]chan Serializable, math.MaxUint16),
	}

	writer := make(chan Serializable, 1500)

	return startReader(read, callbacks, deserialize, writer), startWriter(write, writer, callbacks)
}

func startReader(r ReadHalf, callbacks callbackRingBuf, deserialize Deserializer, writer chan Serializable) RequestReplyReadHandle {
	reader := make(chan Serializable, 1500)

	go func() {
		for {
			msg, _ := r.Read()
			reader <- msg
		}
	}()

	return RequestReplyReadHandle{
		callbacks,
		writer,
		deserialize,
		reader,
	}
}

func startWriter(w WriteHalf, writer chan Serializable, callbacks callbackRingBuf) RequestReplyWriteHandle {

	go func() {
		for {
			req := <-writer

			if err := w.Write(req); err != nil {
				panic("Panic during writing")
			}
		}
	}()

	return RequestReplyWriteHandle{
		callbacks,
		0,
		writer,
	}
}

func (r *RequestReplyReadHandle) Read() interface{} {
	for {
		msg := <-r.readerChan

		switch msg.(type) {

		case *notifyMessageType:
			return NotifyMessageRequestType{
				Msg: msg.(*notifyMessageType).msg,
			}
		case *requestMessageType:
			frame := msg.(*requestMessageType).msg
			id := msg.(*requestMessageType).id

			c := make(chan Serializable)

			go func() {
				res := <-c

				r.writerChan <- &responseMessageType{
					id:  id,
					msg: res,
				}
			}()

			return RequestMessageRequestType{
				Msg:          frame,
				ResponseChan: c,
			}
		case *responseMessageType:
			id := msg.(*responseMessageType).id

			c := r.callbacks.get(id)

			go func() {
				c <- msg.(*responseMessageType).msg
			}()

			r.callbacks.delete(id)
		}

	}
}

func (w *RequestReplyWriteHandle) Notify(req Serializable) {
	notifyMsg := notifyMessageType{
		msg: req,
	}

	w.writerChan <- &notifyMsg
}

func (w *RequestReplyWriteHandle) Request(req Serializable) chan Serializable {
	c := make(chan Serializable)

	messageType := requestMessageType{
		id:  w.registerCallback(c),
		msg: req,
	}

	w.writerChan <- &messageType

	return c
}

func (w *RequestReplyWriteHandle) registerCallback(c chan Serializable) uint16 {
	msgId := w.msgCounter

	for {
		if !w.callbacks.exists(msgId) {
			break
		}

		time.Sleep(1 * time.Microsecond)
	}

	w.callbacks.put(msgId, c)

	if w.msgCounter == (math.MaxUint16 - 1) {
		w.msgCounter = 0
	} else {
		w.msgCounter = w.msgCounter + 1

	}

	return msgId
}
