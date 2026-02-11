package main

import dbconnpool "github.com/rautNishan/db-conn-pool"

func main() {
	db := dbconnpool.Connect(&dbconnpool.Options{
		User:     "postgres",
		Password: "root",
		Database: "api-test",
	})
	db.Ping()
}
