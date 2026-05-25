package dbconnpool

type DbPool struct {
	minConn uint32
	maxConn uint32
}

func Init() {

}

func (dbpool *DbPool) GetConnection() {}
