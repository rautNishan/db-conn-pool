package dbconnpool

import (
	"os"
	"testing"
)

var testConfig = Config{
	Netwrok:          "tcp",
	Address:          "localhost:5432",
	User:             "postgres",
	Password:         "postgres",
	Database:         "test",
	MinConn:          2,
	MaxConn:          10,
	IdealConnTimeOut: 10,
}

func TestMain(m *testing.M) {
	os.Exit(m.Run())
}
