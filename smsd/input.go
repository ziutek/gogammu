package main

import (
	"bufio"
	"github.com/ziutek/mymysql/autorc"
	_ "github.com/ziutek/mymysql/native"
	"io"
	"log"
	"net"
	"strconv"
	"strings"
	"time"
)

// Message format (lines ended by CR or CRLF):
// FROM                                - symbol of source (<=16B)
// PHONE1[=DSTID1] PHONE2[=DSTID2] ... - list of phone numbers and dstIds
// Lines that contain optional parameters, one parameter per line: NAME or
// NAME VALUE. Implemented parameters:
// report        - report required
// delede        - delete message after sending (wait for reports, if required)
//               - empty line
// Message body

// You can use optional dstIds to link recipients with your other data in db.

// Input represents source of messages
type Input struct {
	smsd                           *SMSd
	db                             *autorc.Conn
	knownSrc                       []string
	proto, addr                    string
	ln                             net.Listener
	outboxInsert, recipientsInsert autorc.Stmt
}

func NewInput(smsd *SMSd, proto, addr string, cfg *Config) *Input {
	in := new(Input)
	in.smsd = smsd
	in.db = autorc.New(
		cfg.Db.Proto, cfg.Db.Saddr, cfg.Db.Daddr,
		cfg.Db.User, cfg.Db.Pass, cfg.Db.Name,
	)
	in.db.Raw.Register(setNames)
	in.db.Raw.Register(createOutbox)
	in.db.Raw.Register(createRecipients)
	in.proto = proto
	in.addr = addr
	in.knownSrc = cfg.Source
	return in
}

const outboxInsert = `INSERT
	` + outboxTable + `
SET
	time=?,
	src=?,
	report=?,
	del=?,
	body=?
`

const recipientsInsert = `INSERT ` + recipientsTable + ` SET
	msgId=?,
	number=?,
	dstId=?
`

func (in *Input) handle(c net.Conn) {
	if !prepareOnce(in.db, &in.outboxInsert, outboxInsert) {
		return
	}
	if !prepareOnce(in.db, &in.recipientsInsert, recipientsInsert) {
		return
	}
	r := bufio.NewReader(c)
	from, ok := readLine(r)
	if !ok {
		return
	}
	i := 0
	for i < len(in.knownSrc) && in.knownSrc[i] != from {
		i++
	}
	if i == len(in.knownSrc) {
		log.Println("Unknown source:", from)
		return
	}
	tels, ok := readLine(r)
	if !ok {
		return
	}
	// Read options until first empty line
	var del, report bool
	for {
		l, ok := readLine(r)
		if !ok {
			return
		}
		if l == "" {
			break
		}
		switch l {
		case "report":
			report = true
		case "delete":
			del = true
		}
	}
	buf := make([]byte, 1024)
	n, err := io.ReadFull(r, buf)
	if err != nil && err != io.ErrUnexpectedEOF && err != io.EOF {
		log.Print("Can't read message body: ", err)
		return
	}
	// Insert message into Outbox
	_, res, err := in.outboxInsert.Exec(time.Now(), from, report, del, buf[:n])
	if err != nil {
		log.Printf("Can't insert message from %s into Outbox: %s", from, err)
		return
	}
	msgId := uint32(res.InsertId())
	// Save recipients for this message
	for _, dst := range strings.Split(tels, " ") {
		d := strings.SplitN(dst, "=", 2)
		num := d[0]
		if !checkNumber(num) {
			log.Printf("Bad phone number: '%s' for message #%d.", msgId)
			continue
		}
		var dstId uint64
		if len(d) == 2 {
			dstId, err = strconv.ParseUint(d[1], 0, 32)
			if err != nil {
				dstId = 0
				log.Printf("Bad dstId=`%s` for number %s: %s", d[1], num, err)
			}
		}
		_, _, err = in.recipientsInsert.Exec(msgId, num, uint32(dstId))
		if err != nil {
			log.Printf("Can't insert phone number %s into Recipients: %s", num, err)
		}
	}
	// Inform SMSd about new message
	in.smsd.NewMsg()
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
