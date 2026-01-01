package protocol

import (
	"encoding/binary"
	"io"
	"net"
)

func WriteMessage(conn net.Conn, msg []byte) error {
	msgLength := uint32(len(msg))
	lengthBuff := make([]byte, 4)
	binary.BigEndian.PutUint32(lengthBuff, msgLength)

	if _, err := conn.Write(lengthBuff); err != nil {
		return err
	}
	if _, err := conn.Write(msg); err != nil {
		return err
	}
	return nil
}

func ReadMessage(conn net.Conn) ([]byte, error) {

	lengthBuff := make([]byte, 4)
	if _, err := io.ReadFull(conn, lengthBuff); err != nil {
		return nil, err
	}

	msgLen := binary.BigEndian.Uint32(lengthBuff)
	if msgLen == 0 {
		return nil, nil
	}

	msg := make([]byte, msgLen)
	if _, err := io.ReadFull(conn, msg); err != nil {
		return nil, err
	}

	return msg, nil
}
