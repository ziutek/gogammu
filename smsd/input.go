package main

import (
	"bufio"
	"errors"
	"github.com/ziutek/mymysql/autorc"
	_ "github.com/ziutek/mymysql/native"
	"io"
	"log"
	"net"
)

// Input represents possible source of messages
type Input struct {
	db          *autorc.Conn
	stmt        autorc.Stmt
	proto, addr string
	ln          net.Listener
}

func NewInput(proto, addr string, dbCfg DbCfg) *Input {
	in := new(Input)
	in.db = autorc.New(
		dbCfg.Proto, dbCfg.Saddr, dbCfg.Daddr,
		dbCfg.User, dbCfg.Pass, dbCfg.Db,
	)
	in.db.Raw.Register(setNames)
	in.db.Raw.Register(createOutbox)
	in.proto = proto
	in.addr = addr
	return in
}

func readLine(r *bufio.Reader) (string, bool) {
	l, isPrefix, err := r.ReadLine()
	if err != nil && isPrefix {
		err = errors.New("line too long")
	}
	if err != nil {
		log.Print("Can't read line from input: ", err)
		return "", false
	}
	return string(l), true
}

const outboxInsert = "insert " + outboxTable + " values (0, now(), ?, ?, ?)"

func (in *Input) handle(c net.Conn) {
	if !prepareOnce(in.db, &in.stmt, outboxInsert) {
		return
	}
	r := bufio.NewReader(c)
	from, ok := readLine(r)
	if !ok {
		return
	}
	tels, ok := readLine(r)
	if !ok {
		return
	}
	// Skip following lines until first empty line
	for {
		l, ok := readLine(r)
		if !ok {
			return
		}
		if l == "" {
			break
		}
	}
	buf := make([]byte, 1024)
	n, err := io.ReadFull(r, buf)
	if err != nil && err != io.ErrUnexpectedEOF && err != io.EOF {
		log.Print("Can't read message body: ", err)
		return
	}
	if _, _, err := in.stmt.Exec(from, tels, buf[:n]); err != nil {
		log.Printf("Can't insert message from %s into Outbox", from)
		return
	}
}

func (in *Input) loop() {
	for {
		c, err := in.ln.Accept()
		if err != nil {
			log.Print("Can't accept connection: ", err)
			if e, ok := err.(net.Error); ok && e.Temporary() {
				continue
			}
			return
		}
		go in.handle(c)
	}
}

func (in *Input) Start() error {
	var err error
	log.Println("Listen on:", in.proto, in.addr)
	in.ln, err = net.Listen(in.proto, in.addr)
	if err != nil {
		return err
	}
	go in.loop()
	return nil
}

func (in *Input) Stop() error {
	return in.ln.Close()
}

func (in *Input) String() string {
	return in.proto + ":" + in.addr
}
