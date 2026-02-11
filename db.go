package dbconnpool

import (
	"context"
)

type DB struct {
	*BaseDb
	ctx context.Context
}

func Connect(opt *Options) *DB {
	opt.init()
	return newDb(
		context.Background(),
		&BaseDb{
			opt: opt,
		},
	)
}

func newDb(ctx context.Context, basedb *BaseDb) *DB {
	db := &DB{
		BaseDb: basedb.clone(),
		ctx:    ctx,
	}
	return db
}
