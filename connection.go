package dbconnpool

import (
	"encoding/binary"
	"fmt"
	"net"
)

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
	conn           net.Conn
	BackendKeyData BeKeyData
	Params         []string
	TxStatus       byte
}

func Connect(network, addr, user, database string) (*Conn, error) {
	conn, err := net.Dial(network, addr)
	if err != nil {
		return nil, err
	}
	returnConn := &Conn{conn: conn} //No need to expose this in client side
	_, err = conn.Write(StartUp(user, database))
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
			authType := binary.BigEndian.Uint32(msg.Payload)
			switch AuthType(authType) {
			case AuthenticationOk:
				fmt.Println("Auth okay")
			case AuthenticationKerberosV5:
				continue //Need to implement later
			default:
				return nil, fmt.Errorf("unsupported auth type: %d", authType)
			}
		case byte(ParameterStatus):
			str := string(msg.Payload)
			returnConn.Params = append(returnConn.Params, str)
		case byte(BackendKeyData):
			returnConn.BackendKeyData.ProcessID = binary.BigEndian.Uint32(msg.Payload[:4])
			returnConn.BackendKeyData.SecretKey = binary.BigEndian.Uint32(msg.Payload[4:])
		case byte(ReadyForQuery):
			value := msg.Payload[0]
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
