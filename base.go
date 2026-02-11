package dbconnpool

import "fmt"

type BaseDb struct {
	opt *Options
}

func (baseDb *BaseDb) clone() *BaseDb {
	return &BaseDb{
		opt: baseDb.opt,
	}
}

func (baseDb *BaseDb) Ping() {
	fmt.Println("HEHE")
}
