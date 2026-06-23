package dbconnpool

import (
	"context"
	"crypto/hmac"
	"crypto/pbkdf2"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type Config struct {
	Address          string
	Netwrok          string
	User             string
	Password         string
	Database         string
	MinConn          uint32
	MaxConn          uint32
	IdealConnTimeOut time.Duration //in Seconds
}

type DbPool struct {
	totalConn  uint32
	idelConn   chan *Conn
	activeConn map[*Conn]struct{}
	config     Config
	mutx       sync.Mutex
}

var ErrMaxConnection = errors.New("max connection reached")

func Init(config Config) (*DbPool, error) {
	if config.MinConn == 0 {
		config.MinConn = 1
	}
	if config.MaxConn == 0 {
		config.MaxConn = 10
	}

	if config.MinConn > config.MaxConn {
		return nil, fmt.Errorf("Min is greater than max conn")
	}

	pool := &DbPool{
		idelConn:   make(chan *Conn, config.MaxConn),
		activeConn: make(map[*Conn]struct{}),
		config:     config,
	}
	if pool.config.IdealConnTimeOut == 0 {
		pool.config.IdealConnTimeOut = 60 * time.Second
	} else {
		pool.config.IdealConnTimeOut = pool.config.IdealConnTimeOut * time.Second
	}
	//First make connection of min size (Lazy loading)
	for i := 0; i < int(config.MinConn); i++ {
		conn, err := pool.createConnect()
		if err != nil {
			return nil, err
		}
		pool.idelConn <- conn
	}
	go pool.listenToTimeOuts()
	return pool, nil
}

func (DbPool *DbPool) createConnect() (*Conn, error) {
	atomic.AddUint32(&DbPool.totalConn, 1)
	if DbPool.totalConn > DbPool.config.MaxConn {
		atomic.AddUint32(&DbPool.totalConn, ^uint32(0))
		return nil, ErrMaxConnection
	}
	conn, err := net.Dial(DbPool.config.Netwrok, DbPool.config.Address)
	if err != nil {
		return nil, err
	}
	returnConn := &Conn{netConn: conn} //No need to expose this in client side
	_, err = returnConn.netConn.Write(StartUp(DbPool.config.User, DbPool.config.Database))
	if err != nil {
		return nil, err
	}
	for {
		msg, err := returnConn.getMessage()
		if err != nil {
			conn.Close()
			return nil, err
		}
		fmt.Println("message type: ", string(msg.Type))
		switch msg.Type {
		case byte(MsgAuthRequest):
			fmt.Println("Length of payload: ", len(msg.Payload))
			authType := binary.BigEndian.Uint32(msg.Payload[:4])
			authOffset := 4
			fmt.Println("Authtype: ", authType)
			switch AuthType(authType) {
			case AuthenticationOk:
				fmt.Println("Auth okay")
			case AuthenticationKerberosV5:
				continue //Need to implement later
			case AuthenticationSASL: //[SASL] https://datatracker.ietf.org/doc/html/rfc4422
				fmt.Println("AuthenticationSASL")
				mechanisms := parseSaslMechanism(msg.Payload[authOffset:])
				if len(mechanisms) == 0 {
					return nil, fmt.Errorf("no supported SASL mechanism in: %v", mechanisms)
				}
				//[SCRAM] https://datatracker.ietf.org/doc/html/rfc5802
				mechanism := mechanisms[0]
				fmt.Println(mechanism)
				//Need to better implement this
				if mechanism == "SCRAM-SHA-256" {
					//https://www.postgresql.org/docs/current/sasl-authentication.html
					clientFirstMsg, clientFirstMsgBare, clientNonce, err := buildClientFirstMessage(DbPool.config.User, false)
					if err != nil {
						return nil, fmt.Errorf("failed to build client-first-message: %w", err)
					}
					returnConn.saslState = SASLState{
						clientFirstMessageBare: clientFirstMsgBare,
						clientNonce:            clientNonce,
					}
					fmt.Println(clientFirstMsgBare, clientNonce)
					msgLen := 4 + // length field itself
						len(mechanism) + 1 + // mechanism name + null terminator
						4 + // client-first-message length field
						len(clientFirstMsg) // client-first-message bytes
					buff := make([]byte, 0, 1+msgLen)
					buff = append(buff, 'p')
					buff = binary.BigEndian.AppendUint32(buff, uint32(msgLen))
					buff = append(buff, []byte(mechanism)...)
					buff = append(buff, '\x00')
					buff = binary.BigEndian.AppendUint32(buff, uint32(len(clientFirstMsg)))
					buff = append(buff, []byte(clientFirstMsg)...)
					if _, err := returnConn.netConn.Write(buff); err != nil {
						DbPool.closeConn(returnConn)
						return nil, fmt.Errorf("failed to send SASLInitialResponse: %w", err)
					}
				}
			//https://datatracker.ietf.org/doc/html/rfc5802#section-3
			//https://datatracker.ietf.org/doc/html/rfc5802#section-5
			//https://csb.stevekerrison.com/post/2022-05-scram-detail/
			case AuthenticationSASLContinue:
				fmt.Println("Continue SASL")
				saslData := msg.Payload[authOffset:]
				serverFirstMessage := string(saslData)
				fmt.Println("Server first data: ", serverFirstMessage)
				parts := strings.Split(serverFirstMessage, ",")
				fmt.Println("Parts: ", parts)
				var serverNonce, saltB64 string
				var iteration int
				for _, part := range parts {
					switch {
					case strings.HasPrefix(part, "r="):
						serverNonce = strings.TrimPrefix(part, "r=")
					case strings.HasPrefix(part, "s="):
						saltB64 = strings.TrimPrefix(part, "s=")
					case strings.HasPrefix(part, "i="):
						iteration, _ = strconv.Atoi(strings.TrimPrefix(part, "i="))
					}
				}
				fmt.Println(serverNonce, saltB64, iteration)
				fmt.Println(returnConn.saslState.clientNonce)
				if !strings.HasPrefix(serverNonce, returnConn.saslState.clientNonce) {
					DbPool.closeConn(returnConn)
					return nil, fmt.Errorf("server nonce does not start with client nonce")
				}
				salt, err := base64.StdEncoding.DecodeString(saltB64)
				if err != nil {
					return nil, fmt.Errorf("failed to decode salt: %w", err)
				}
				fmt.Println(salt)

				//Following the steps from (https://datatracker.ietf.org/doc/html/rfc5802#section-3)
				saltedPass, err := pbkdf2.Key(sha256.New, DbPool.config.Password, salt, iteration, 32)
				if err != nil {
					return nil, fmt.Errorf("Error while getting salted pass: %w", err)
				}
				fmt.Println("Salted pass: ", saltedPass)
				returnConn.saslState.saltedPass = saltedPass
				clientKey := hmac.New(sha256.New, saltedPass)
				clientKey.Write([]byte("Client Key"))
				clientKeySum := clientKey.Sum(nil)

				storedKey := sha256.Sum256(clientKeySum)
				//https://datatracker.ietf.org/doc/html/rfc5802#section-7
				clientFinalWithoutProof := "c=biws,r=" + serverNonce
				authMessage := returnConn.saslState.clientFirstMessageBare + "," +
					serverFirstMessage + "," +
					clientFinalWithoutProof
				returnConn.saslState.authMessage = authMessage
				sig := hmac.New(sha256.New, storedKey[:])
				sig.Write([]byte(authMessage))
				clientSignature := sig.Sum(nil)
				clientProof := make([]byte, len(clientKeySum))
				for i := range clientKeySum {
					clientProof[i] = clientKeySum[i] ^ clientSignature[i]
				}
				clientFinalMessage := clientFinalWithoutProof + ",p=" +
					base64.StdEncoding.EncodeToString(clientProof)
				body := []byte(clientFinalMessage)
				length := uint32(4 + len(body)) // 4 = the length field counts itself, NOT the tag

				buf := make([]byte, 0, 1+4+len(body))
				buf = append(buf, 'p')                           // tag
				buf = binary.BigEndian.AppendUint32(buf, length) // length, big-endian
				buf = append(buf, body...)                       // payload
				if _, err := returnConn.netConn.Write(buf); err != nil {
					return nil, fmt.Errorf("failed to write SASL response: %w", err)
				}
			case AuthenticationSASLFinal:
				completed := binary.BigEndian.Uint32(msg.Payload[0:4])
				fmt.Println(completed)
				additionalData := msg.Payload[4:]
				serverSigB64 := strings.TrimPrefix(string(additionalData), "v=")
				receivedServerSig, err := base64.StdEncoding.DecodeString(serverSigB64)
				if err != nil {
					return nil, fmt.Errorf("failed to decode server signature: %w", err)
				}
				serverKeyMAC := hmac.New(sha256.New, returnConn.saslState.saltedPass)
				serverKeyMAC.Write([]byte("Server Key"))
				serverKey := serverKeyMAC.Sum(nil)

				serverSigMAC := hmac.New(sha256.New, serverKey)
				serverSigMAC.Write([]byte(returnConn.saslState.authMessage))
				expectedServerSig := serverSigMAC.Sum(nil)

				if !hmac.Equal(receivedServerSig, expectedServerSig) {
					DbPool.closeConn(returnConn)
					return nil, fmt.Errorf("server signature mismatch — possible MITM or wrong server")
				}
				returnConn.saslState = SASLState{}
				fmt.Println("Server verified")

			default:
				DbPool.closeConn(returnConn)
				return nil, fmt.Errorf("unsupported auth type: %d", authType)
			}
		case byte(ParameterStatus):
			str := string(msg.Payload)
			returnConn.params = append(returnConn.params, str)
		case byte(BackendKeyData):
			returnConn.backendKeyData.ProcessID = binary.BigEndian.Uint32(msg.Payload[:4])
			returnConn.backendKeyData.SecretKey = binary.BigEndian.Uint32(msg.Payload[4:])
		case byte(ReadyForQuery):
			value := msg.Payload[0]
			returnConn.txStatus = value
			fmt.Println("Return cunnection tx status: ", string(returnConn.txStatus))
			return returnConn, nil
		case byte(CommandComplete):
			fmt.Println("Command Complete")
		case byte(NoticeResponse):
			fmt.Println("Notice Response")
		case byte(ErrorResponse):
			DbPool.closeConn(returnConn)
			return nil, fmt.Errorf("Error response in message")
		default:
			// If the frontend does not support the authentication method requested by the server,
			// then it should immediately close the connection.
			DbPool.closeConn(returnConn)
			return nil, fmt.Errorf("unsupported message type: %d", msg.Type)
		}
	}

}

func (DbPool *DbPool) GetConnetion(ctx context.Context) (*Conn, error) {
	var conn *Conn
	var err error

	select {
	case conn = <-DbPool.idelConn:
	default:
		conn, err = DbPool.createConnect()
		if err != nil {
			if errors.Is(err, ErrMaxConnection) {
				select {
				case conn = <-DbPool.idelConn:
				case <-ctx.Done(): //Limits how long the caller waits to acquire a connection
					return nil, fmt.Errorf("connection acquisition timed out: %w", ctx.Err())
				}
			} else {
				return nil, err
			}
		}
	}

	if !conn.isAlive() {
		//Close the connection
		DbPool.closeConn(conn)
		conn, err = DbPool.createConnect()
		if err != nil {
			return DbPool.GetConnetion(ctx)
		}
	}
	//Clouse (Whats being assigned) attatch this function to connection type
	// So Release can have these
	conn.release = func(healthy bool) {
		removed := DbPool.removeConnFromActive(conn)
		if healthy {
			if removed {
				conn.status = statusIdeal
				DbPool.idelConn <- conn
			}
			return
		} else {
			DbPool.closeConn(conn)
			return
		}
	}

	DbPool.putInActiveConn(conn)
	conn.addTimeOuts(DbPool.config.IdealConnTimeOut)
	return conn, nil
}

func (DbPool *DbPool) closeConn(conn *Conn) error {
	atomic.AddUint32(&DbPool.totalConn, ^uint32(0)) //-1
	conn.close.Store(true)
	err := conn.netConn.Close()
	if err != nil {
		return err
	}
	return nil
}

func (DbPool *DbPool) removeConnFromActive(conn *Conn) bool {
	DbPool.mutx.Lock()
	defer DbPool.mutx.Unlock()
	_, ok := DbPool.activeConn[conn]
	conn.status = statusIdeal
	if !ok {
		return false
	}
	delete(DbPool.activeConn, conn)
	return true
}

func (DbPool *DbPool) putInActiveConn(conn *Conn) {
	DbPool.mutx.Lock()
	defer DbPool.mutx.Unlock()
	conn.status = statusAcquired
	DbPool.activeConn[conn] = struct{}{}
}

func (DbPool *DbPool) listenToTimeOuts() {
	interval := DbPool.config.IdealConnTimeOut / 2
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for range ticker.C {
		now := time.Now()
		var expired []*Conn
		DbPool.mutx.Lock()
		for key := range DbPool.activeConn {
			//If it is still querying no need for release
			if key.status == statusAcquired && now.After(key.timeOut) {
				expired = append(expired, key)
			}
		}
		DbPool.mutx.Unlock()
		for _, c := range expired {
			c.Release()
		}

	}

}
