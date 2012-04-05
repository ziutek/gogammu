package main

import (
	"github.com/ziutek/mymysql/autorc"
	"log"
	"os"
	"time"
)

func prepareOnce(db *autorc.Conn, stmt *autorc.Stmt, sql string) bool {
	err := db.PrepareOnce(stmt, sql)
	if err == nil {
		return true
	}
	log.Printf("Can't prepare `%s`: %s", sql, err)
	if !autorc.IsNetErr(err) {
		os.Exit(1)
	}
	return false
}

func isGammuError(e error) bool {
	if e == nil {
		return false
	}
	log.Println("Can't communicate with phone:", e)
	time.Sleep(60 * time.Second)
	return true
}
