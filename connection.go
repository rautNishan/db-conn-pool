package dbconnpool

import (
	"encoding/binary"
	"fmt"
	"net"

	"github.com/rautNishan/db-conn-pool/protocol"
)

type MessageType byte

type Conn struct {
	conn net.Conn
}

type Message struct {
	Type    byte
	Length  uint32
	payload []byte
}

const (
	MsgAuthRequest MessageType = 'R'
)

func Connect(network, addr, user, database string) (*Conn, error) {
	conn, err := net.Dial(network, addr)
	if err != nil {
		fmt.Println(err)
		return nil, err
	}
	fmt.Println(conn)
	n, err := conn.Write(protocol.StartUp(user, database))
	if err != nil {
		return nil, err
	}
	fmt.Println("Write number of bytes: ", n)

	for {
		msg, err := getMessage(conn)
		if err != nil {
			return nil, err
		}
		fmt.Println("Message type: ", string(msg.Type))
		fmt.Println("Payload: ", msg.payload)
		break
	}
	return &Conn{conn: conn}, nil
}
func getMessage(conn net.Conn) (*Message, error) {
	header, err := ReadExactly(conn, 5)

	if err != nil {
		return nil, err
	}
	msgType := header[0]
	payloadLen := binary.BigEndian.Uint32(header[1:5])
	payload, err := ReadExactly(conn, int(payloadLen)-4) // length includes the 4-byte length field
	if err != nil {
		return nil, err
	}
	return &Message{Type: msgType, payload: payload, Length: payloadLen}, nil
}
func ReadExactly(conn net.Conn, n int) ([]byte, error) {
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
