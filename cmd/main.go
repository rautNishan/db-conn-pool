package main

import (
	"fmt"

	dbconnpool "github.com/rautNishan/db-conn-pool"
)

func main() {
	conn, err := dbconnpool.Connect("tcp", "localhost:5432", "postgres", "test")
	if err != nil {
		fmt.Println(err)
	}
	fmt.Println("This is connection: ", conn)
}
