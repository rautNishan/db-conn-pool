package dbconnpool

type Config struct {
	Address  string
	Netwrok  string
	User     string
	Password string
	Database string
	MinConn  uint32
	MaxConn  uint32
}

type DbPool struct {
	idelConn   chan *Conn
	activeConn map[*Conn]struct{}
	totalConn  uint32
}

func Init(config Config) (*DbPool, error) {
	pool := &DbPool{
		idelConn:   make(chan *Conn, config.MaxConn),
		activeConn: make(map[*Conn]struct{}),
	}

	//First make connection of min size (Lazy loading)
	for i := 0; i < int(config.MinConn); i++ {
		conn, err := connect(config.Netwrok, config.Address, config.User, config.Database)
		if err != nil {
			return nil, err
		}
		pool.idelConn <- conn
	}
	return pool, nil
}

func (DbPool *DbPool) Query() {
}
func (DbPool *DbPool) GetConnection() {
}
