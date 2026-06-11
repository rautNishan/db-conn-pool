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
	pool := &DbPool{
		idelConn:   make(chan *Conn, config.MaxConn),
		activeConn: make(map[*Conn]struct{}),
		config:     config,
	}
	if pool.config.IdealConnTimeOut == 0 {
		pool.config.IdealConnTimeOut = 60 * time.Second
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
	if DbPool.totalConn >= DbPool.config.MaxConn {
		return nil, ErrMaxConnection
	}
	conn, err := net.Dial(DbPool.config.Netwrok, DbPool.config.Address)
	if err != nil {
		return nil, err
	}
	returnConn := &Conn{NetConn: conn} //No need to expose this in client side
	_, err = returnConn.NetConn.Write(StartUp(DbPool.config.User, DbPool.config.Database))
	if err != nil {
		return nil, err
	}
	for {
		msg, err := returnConn.getMessage()
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
			returnConn.readyForQuery = true
			atomic.AddUint32(&DbPool.totalConn, 1)
			return returnConn, nil
		case byte(ErrorResponse):
			fmt.Println("Error response message")
			return nil, fmt.Errorf("Error response in message")
		default:
			// If the frontend does not support the authentication method requested by the server,
			// then it should immediately close the connection.
			returnConn.NetConn.Close()
			return nil, fmt.Errorf("unsupported message type: %d", msg.Type)
		}
	}
}

func (DbPool *DbPool) GetConnetion() (*Conn, error) {
	var conn *Conn
	var err error
	fmt.Println("len: ", len(DbPool.idelConn))
	if len(DbPool.idelConn) == 0 {
		conn, err = DbPool.createConnect()
		if err != nil {
			if errors.Is(err, ErrMaxConnection) {
				conn = <-DbPool.idelConn
			} else {
				return nil, err
			}
		}
	} else {
		conn = <-DbPool.idelConn
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
		DbPool.removeConnFromActive(conn)
		if healthy {
			DbPool.idelConn <- conn
		} else {
			DbPool.closeConn(conn)
		}
	}
	DbPool.putInActiveConn(conn)
	conn.addTimeOuts(DbPool.config.IdealConnTimeOut)
	return conn, nil
}

func (DbPool *DbPool) closeConn(conn *Conn) error {
	err := conn.NetConn.Close()
	if err != nil {
		return err
	}
	atomic.AddUint32(&DbPool.totalConn, ^uint32(0)) //-1
	return nil
}

func (DbPool *DbPool) removeConnFromActive(conn *Conn) {
	DbPool.mutx.Lock()
	delete(DbPool.activeConn, conn)
	DbPool.mutx.Unlock()
}

func (DbPool *DbPool) putInActiveConn(conn *Conn) {
	DbPool.mutx.Lock()
	DbPool.activeConn[conn] = struct{}{}
	DbPool.mutx.Unlock()
}

func (DbPool *DbPool) listenToTimeOuts() {
	interval := time.Second * DbPool.config.IdealConnTimeOut / 2
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for range ticker.C {
		fmt.Println("Checking in interval of: ", interval)
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
			fmt.Println("Freeing the connection")
			c.Release()
		}

	}

}
