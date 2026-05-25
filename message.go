package dbconnpool

import (
	"encoding/binary"
	"fmt"
	"net"
)

type Message struct {
	Type    byte
	Length  uint32
	Payload []byte
}
type MessageType byte

// Message pg sends
const (
	MsgAuthRequest  MessageType = 'R'
	ParameterStatus MessageType = 'S'
	BackendKeyData  MessageType = 'K'
	ReadyForQuery   MessageType = 'Z'
)

func getMessage(conn net.Conn) (Message, error) {
	header, err := readExactly(conn, 5)

	if err != nil {
		return Message{}, err
	}
	msgType := header[0]
	payloadLen := binary.BigEndian.Uint32(header[1:5])
	payload, err := readExactly(conn, int(payloadLen)-4) // length includes the 4-byte length field
	if err != nil {
		return Message{}, err
	}
	return Message{Type: msgType, Payload: payload, Length: payloadLen}, nil
}

func readExactly(conn net.Conn, n int) ([]byte, error) {
	buff := make([]byte, n)
	total := 0
	for total < n {
		read, err := conn.Read(buff[total:])
		if err != nil {
			fmt.Println("Erro while reading: ", err)
			return nil, err
		}
		total += read
	}
	return buff, nil
}
