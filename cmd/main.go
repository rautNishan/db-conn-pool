package main

import (
	"context"
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
		IdealConnTimeOut: 10,
		Password:         "test",
	})
	if err != nil {
		fmt.Println("Error in main: ", err)
		return
	}
	conn, err := pool.GetConnetion(context.Background()) //Todo need to fix
	if err != nil {
		fmt.Println("First Error: ", err)
	}
	conn.Query("SELECT 1")
	fmt.Println("Command sent")

}
