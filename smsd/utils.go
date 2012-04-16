package main

import (
	"bufio"
	"errors"
	"github.com/ziutek/mymysql/autorc"
	"log"
	"os"
	"time"
	"unicode"
)

type event struct{}

func readLine(r *bufio.Reader) (string, bool) {
	l, isPrefix, err := r.ReadLine()
	if err != nil && isPrefix {
		err = errors.New("line too long")
	}
	if err != nil {
		log.Print("Can't read line: ", err)
		return "", false
	}
	return string(l), true
}

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

func checkNumber(num string) bool {
	if num[0] == '+' {
		num = num[1:]
	}
	for _, r := range num {
		if !unicode.IsDigit(r) {
			return false
		}
	}
	return true
}
