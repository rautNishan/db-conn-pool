package dbconnpool

import (
	"net"
	"time"
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
	TxStatus       byte
	Params         []string
	BackendKeyData BeKeyData
	NetConn        net.Conn //Only for testing
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
