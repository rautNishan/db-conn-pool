package main

import (
	"fmt"

	dbconnpool "github.com/rautNishan/db-conn-pool"
)

func main() {
	pool, err := dbconnpool.Init(dbconnpool.Config{
		Netwrok:          "tcp",
		Address:          "localhost:5432",
		User:             "postgres",
		Database:         "test",
		MinConn:          1,
		MaxConn:          1,
		IdealConnTimeOut: 60,
	})
	if err != nil {
		fmt.Println(err)
	}
	conn, err := pool.GetConnetion()
	if err != nil {
		fmt.Println("First Error: ", err)
	}
	conn.Query("SELECT 1")
	fmt.Println("Command sent")

}
