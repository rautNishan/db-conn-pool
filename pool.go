package dbconnpool

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"net"
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
			fmt.Println("Authtype: ", authType)
			switch AuthType(authType) {
			case AuthenticationOk:
				fmt.Println("Auth okay")
			case AuthenticationKerberosV5:
				continue //Need to implement later
			case AuthenticationSASL:
				fmt.Println("AuthenticationSASL")
				mechanisms := parseSaslMechanism(msg.Payload)
				fmt.Println("Mechanisms: ", mechanisms)
				if len(mechanisms) == 0 {
					return nil, fmt.Errorf("no supported SASL mechanism in: %v", mechanisms)
				}
				fmt.Println("Mechanism: ", mechanisms[0])
				payload := []byte(mechanisms[0])
				fmt.Println("This is len of payload: ", len(payload))
				fmt.Println("Payload: ", string(payload))
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
