package dbconnpool

import (
	"encoding/binary"
	"fmt"
	"net"

	"github.com/rautNishan/db-conn-pool/protocol"
)

type MessageType byte
type AuthType uint32
type Conn struct {
	conn             net.Conn
	BackendProcessId uint32
	Params           []string
	TxStatus         byte
}

type Message struct {
	Type    byte
	Length  uint32
	payload []byte
}

// Message pg sends
const (
	MsgAuthRequest  MessageType = 'R'
	ParameterStatus MessageType = 'S'
	BackendKeyData  MessageType = 'K'
	ReadyForQuery   MessageType = 'Z'
)

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

func Connect(network, addr, user, database string) (*Conn, error) {
	conn, err := net.Dial(network, addr)
	if err != nil {
		return nil, err
	}
	returnConn := &Conn{conn: conn} //No need to expose this in client side
	_, err = conn.Write(protocol.StartUp(user, database))
	if err != nil {
		return nil, err
	}
	for {
		msg, err := getMessage(conn)
		if err != nil {
			conn.Close()
			return nil, err
		}
		switch msg.Type {
		case byte(MsgAuthRequest):
			authType := binary.BigEndian.Uint32(msg.payload)
			switch AuthType(authType) {
			case AuthenticationOk:
				fmt.Println("Auth okay")
			case AuthenticationKerberosV5:
				continue //Need to implement later
			default:
				return nil, fmt.Errorf("unsupported auth type: %d", authType)
			}
		case byte(ParameterStatus):
			str := string(msg.payload)
			returnConn.Params = append(returnConn.Params, str)
		case byte(BackendKeyData):
			backendProcessid := binary.BigEndian.Uint32(msg.payload)
			returnConn.BackendProcessId = backendProcessid
		case byte(ReadyForQuery):
			value := msg.payload[0]
			returnConn.TxStatus = value
			return returnConn, nil
		default:
			// If the frontend does not support the authentication method requested by the server,
			// then it should immediately close the connection.
			conn.Close()
			return nil, fmt.Errorf("unsupported message type: %d", msg.Type)
		}
	}
}

func getMessage(conn net.Conn) (Message, error) {
	header, err := protocol.ReadExactly(conn, 5)

	if err != nil {
		return Message{}, err
	}
	msgType := header[0]
	payloadLen := binary.BigEndian.Uint32(header[1:5])
	payload, err := protocol.ReadExactly(conn, int(payloadLen)-4) // length includes the 4-byte length field
	if err != nil {
		return Message{}, err
	}
	return Message{Type: msgType, payload: payload, Length: payloadLen}, nil
}
