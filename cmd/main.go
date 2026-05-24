package main

import (
	"fmt"

	dbconnpool "github.com/rautNishan/db-conn-pool"
)

func main() {
	_, err := dbconnpool.Connect("tcp", "localhost:5432", "postgres", "test")
	if err != nil {
		fmt.Println(err)
	}
}
