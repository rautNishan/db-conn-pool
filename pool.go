package dbconnpool

import (
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
			authType := binary.BigEndian.Uint32(msg.Payload)
			switch AuthType(authType) {
			case AuthenticationOk:
				fmt.Println("Auth okay")
			case AuthenticationKerberosV5:
				continue //Need to implement later
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

func (DbPool *DbPool) GetConnetion() (*Conn, error) {
	var conn *Conn
	var err error
	select {
	case conn = <-DbPool.idelConn:
	default:
		conn, err = DbPool.createConnect()
		if err != nil {
			if errors.Is(err, ErrMaxConnection) {
				conn = <-DbPool.idelConn
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
			return DbPool.GetConnetion()
		}
	}
	//Clouse (Whats being assigned) attatch this function to connection type
	// So Release can have these
	conn.release = func(healthy bool) {
		removed := DbPool.removeConnFromActive(conn)
		fmt.Println("Is healthy: ", healthy)
		if healthy {
			if removed {
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
	err := conn.netConn.Close()
	if err != nil {
		return err
	}
	return nil
}

func (DbPool *DbPool) removeConnFromActive(conn *Conn) bool {
	DbPool.mutx.Lock()
	_, ok := DbPool.activeConn[conn]
	if !ok {
		return false
	}
	delete(DbPool.activeConn, conn)
	DbPool.mutx.Unlock()
	return true
}

func (DbPool *DbPool) putInActiveConn(conn *Conn) {
	DbPool.mutx.Lock()
	DbPool.activeConn[conn] = struct{}{}
	DbPool.mutx.Unlock()
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
			if now.After(key.timeOut) {
				expired = append(expired, key)
			}
		}
		DbPool.mutx.Unlock()
		for _, c := range expired {
			c.Release()
		}

	}

}
