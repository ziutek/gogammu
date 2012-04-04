package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
)

type DbCfg struct{ Proto, Saddr, Daddr, User, Pass, Db string }

var dbCfg = DbCfg{
	"tcp", "", "127.0.0.1:3306", "testuser", "TestPasswd9", "test",
}

const setNames = "SET NAMES utf8"
const createOutbox = `CREATE TABLE IF NOT EXISTS SMSd_Outbox (
	id   INT UNSIGNED NOT NULL PRIMARY KEY AUTO_INCREMENT,
	time DATETIME NOT NULL,
	src  VARCHAR(16) NOT NULL,
	dst	 VARCHAR(129) NOT NULL,
	body TEXT NOT NULL
) DEFAULT CHARSET=utf8`

var (
	ins  []*Input
	smsd *SMSd
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "Usage: %s LISTEN_ADDR ...\n", os.Args[0])
		os.Exit(1)
	}

	smsd, err := NewSMSd(dbCfg)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}

	ins = make([]*Input, len(os.Args)-1)
	for i, a := range os.Args[1:] {
		proto := "tcp"
		if strings.IndexRune(a, ':') == -1 {
			proto = "unix"
		}
		ins[i] = NewInput(proto, a, dbCfg)
	}

	smsd.Start()
	for _, in := range ins {
		if err := in.Start(); err != nil {
			log.Print("Can't start input thread: ", in)
		}
	}

	sc := make(chan os.Signal, 2)
	signal.Notify(sc, syscall.SIGTERM, syscall.SIGINT)
	<-sc

	for _, in := range ins {
		in.Stop()
	}
	smsd.Stop()
}
