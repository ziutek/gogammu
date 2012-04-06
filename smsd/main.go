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
const outboxTable = "SMSd_Outbox"
const phonesTable = "SMSd_Phones"
const createOutbox = `CREATE TABLE IF NOT EXISTS ` + outboxTable + ` (
	id     int unsigned NOT NULL AUTO_INCREMENT,
	time   datetime NOT NULL,
	src    varchar(16) NOT NULL,
	report boolean NOT NULL,
	del    boolean NOT NULL,
	body   text NOT NULL,
	PRIMARY KEY (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8`
const createPhones = `CREATE TABLE IF NOT EXISTS ` + phonesTable + ` (
	id     int unsigned NOT NULL AUTO_INCREMENT,
	msgId  int unsigned NOT NULL,
	number varchar(16) NOT NULL,
	dstId  int unsigned NOT NULL,
	sent   datetime NOT NULL,
	report datetime NOT NULL,
	PRIMARY KEY (id),
	FOREIGN KEY (msgId) REFERENCES ` + outboxTable + `(id) ON DELETE CASCADE,
	KEY dstId (dstId)
) ENGINE=InnoDB DEFAULT CHARSET=utf8`

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
