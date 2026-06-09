package main

import (
	"fmt"

	dbconnpool "github.com/rautNishan/db-conn-pool"
)

func main() {
	// conn, err := dbconnpool.Connect("tcp", "localhost:5432", "postgres", "test")
	// if err != nil {
	// 	fmt.Println(err)
	// }
	// fmt.Printf("Connection: %+v\n", conn)
	// dbconnpool.SimpleQuery("SELECT 1", conn.NetConn)

	pool, err := dbconnpool.Init(dbconnpool.Config{
		Netwrok:  "tcp",
		Address:  "localhost:5432",
		User:     "postgres",
		Database: "test",
		MinConn:  1,
		MaxConn:  1,
	})
	if err != nil {
		fmt.Println(err)
	}
	conn, err := pool.GetConnetion()
	if err != nil {
		fmt.Println("First Error: ", err)
	}
	fmt.Println("This is connection", conn)
	conn2, err := pool.GetConnetion()
	if err != nil {
		fmt.Println("Second Error: ", err)
	}
	fmt.Println("This is connection", conn2)

}
