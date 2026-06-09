package dbconnpool

import (
	"encoding/binary"
	"fmt"
	"net"
	"time"
)

// Message pg sends
const (
	MsgAuthRequest  MessageType = 'R'
	ParameterStatus MessageType = 'S'
	BackendKeyData  MessageType = 'K'
	ReadyForQuery   MessageType = 'Z'
	ErrorResponse   MessageType = 'E'
)

type Message struct {
	Type    byte
	Length  uint32
	Payload []byte
}
type MessageType byte
type AuthType uint32

// https://www.postgresql.org/docs/current/protocol-message-formats.html
const (
	AuthenticationOk                AuthType = 0
	AuthenticationKerberosV5        AuthType = 2
	AuthenticationCleartextPassword AuthType = 3
	AuthenticationMD5Password       AuthType = 5
	AuthenticationGSS               AuthType = 7
	AuthenticationGSSContinue       AuthType = 8
	AuthenticationSSPI              AuthType = 9
	AuthenticationSASL              AuthType = 10
	AuthenticationSASLContinue      AuthType = 11
	AuthenticationSASLFinal         AuthType = 12
)

type BeKeyData struct {
	ProcessID uint32
	SecretKey uint32
}
type Conn struct {
	TxStatus       byte
	Params         []string
	BackendKeyData BeKeyData
	NetConn        net.Conn //Only for testing
	release        func(healthy bool)
	readyForQuery  bool
}

func (conn *Conn) isAlive() bool {
	conn.NetConn.SetDeadline(time.Now().Add(1 * time.Millisecond))
	defer conn.NetConn.SetReadDeadline(time.Time{})
	buff := make([]byte, 1)
	_, err := conn.NetConn.Read(buff)
	if err != nil {
		//if time out error then conn is stil alive
		if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
			return true
		}
		return false
	}
	return true
}

func (conn *Conn) getMessage() (Message, error) {
	header, err := conn.readExactly(5)

	if err != nil {
		return Message{}, err
	}
	msgType := header[0]
	payloadLen := binary.BigEndian.Uint32(header[1:5])
	payload, err := conn.readExactly(int(payloadLen) - 4) // length includes the 4-byte length field
	if err != nil {
		return Message{}, err
	}
	return Message{Type: msgType, Payload: payload, Length: payloadLen}, nil
}

func (conn *Conn) readExactly(n int) ([]byte, error) {
	buff := make([]byte, n)
	total := 0
	for total < n {
		read, err := conn.NetConn.Read(buff[total:])
		if err != nil {
			fmt.Println("Erro while reading: ", err)
			return nil, err
		}
		total += read
	}
	return buff, nil
}

func (conn *Conn) Release() {
	if conn.readyForQuery {
		conn.readyForQuery = false
		conn.release(true)
		return
	}

	for {
		msg, err := conn.getMessage()
		if err != nil {
			conn.release(false)
			return
		}
		if msg.Type == byte(ReadyForQuery) {
			conn.release(true)
			return
		}
	}
}
