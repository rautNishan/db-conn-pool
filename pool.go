package dbconnpool

type Config struct {
	Address  string
	Netwrok  string
	User     string
	Password string
	Database string
}
type DbPool struct {
	idelConn   []Conn
	totalConn  []Conn
	activeConn []Conn
}

func Init(config Config) {

}

func (dbpool *DbPool) GetConnection() {}

func (DbPool *DbPool) Query() {
}
