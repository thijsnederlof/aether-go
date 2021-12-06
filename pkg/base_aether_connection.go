package pkg

import (
	"bytes"
	"encoding/binary"
	"io"
	"net"
)

type WriteHalf struct {
	conn net.TCPConn
	buf  *bytes.Buffer
}

type ReadHalf struct {
	conn         net.TCPConn
	deserializer Deserializer
	buf          *bytes.Buffer
}

type BaseAetherConnection struct {
	w WriteHalf
	r ReadHalf
}

func NewBaseAetherConnection(conn net.TCPConn, deserialize Deserializer) BaseAetherConnection {
	conn.SetNoDelay(true)

	aetherConn := BaseAetherConnection{
		w: WriteHalf{
			conn,
			bytes.NewBuffer(make([]byte, 4096)),
		},
		r: ReadHalf{
			conn,
			deserialize,
			bytes.NewBuffer(make([]byte, 4096)),
		},
	}

	aetherConn.r.buf.Reset()
	aetherConn.w.buf.Reset()

	return aetherConn
}

func (conn *BaseAetherConnection) Split() (ReadHalf, WriteHalf) {
	return conn.r, conn.w
}

func (w *WriteHalf) Write(msg Serializable) error {
	defer w.buf.Reset()

	// reserve message len
	w.buf.WriteByte(0x00)
	w.buf.WriteByte(0x00)

	msg.Serialize(w.buf)

	frameLen := uint16(w.buf.Len() - 2)

	out := w.buf.Bytes()

	binary.LittleEndian.PutUint16(out[0:2], frameLen)

	_, err := w.conn.Write(out)

	return err
}

func (r *ReadHalf) Read() (Serializable, error) {
	defer r.buf.Reset()

	if err := readUntilBufContainsAtLeast(2, r.buf, r.conn); err != nil {
		return nil, err
	}

	frameLen := binary.LittleEndian.Uint16(r.buf.Next(2))

	if err := readUntilBufContainsAtLeast(frameLen, r.buf, r.conn); err != nil {
		return nil, err
	}

	frame := r.buf.Next(int(frameLen))

	return r.deserializer.Deserialize(frame), nil
}

func readUntilBufContainsAtLeast(size uint16, buf *bytes.Buffer, conn net.TCPConn) error {
	_, err := buf.ReadFrom(io.LimitReader(&conn, int64(size)))

	return err
}
