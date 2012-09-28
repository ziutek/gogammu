package main

import (
	"github.com/ziutek/gogammu"
	"github.com/ziutek/mymysql/autorc"
	_ "github.com/ziutek/mymysql/native"
	"io"
	"log"
	"os"
	"strings"
	"time"
)

type SMSd struct {
	sm *gammu.StateMachine
	db *autorc.Conn

	end, newMsg chan event
	wait        bool

	gammuErrors, gammuConnErrors uint

	sqlNumToId string

	stmtOutboxGet, stmtRecipGet, stmtRecipSent, stmtInboxPut,
	stmtRecipReport, stmtOutboxDel, stmtNumToId autorc.Stmt

	filter  *Filter
	pullInt time.Duration
}

func NewSMSd(db *autorc.Conn, numId, filter string, pullInt time.Duration) *SMSd {
	var err error

	smsd := new(SMSd)
	smsd.sm, err = gammu.NewStateMachine("")
	if err != nil {
		log.Println("Can't create gammu state machine:", err)
		os.Exit(1)
	}

	smsd.pullInt = pullInt
	log.Println("Pull interval:", pullInt)

	if filter != "" {
		smsd.filter, err = NewFilter(filter)
		if err != nil {
			log.Println("Can't setup a filter:", err)
			os.Exit(1)
		}
	}
	log.Println("Filter:", filter)

	smsd.db = db
	smsd.db.Register(setNames)
	smsd.db.Register(createOutbox)
	smsd.db.Register(createRecipients)
	smsd.db.Register(createInbox)
	smsd.db.Register(setLocPrefix)
	smsd.sqlNumToId = numId
	smsd.end = make(chan event)
	smsd.newMsg = make(chan event, 1)
	return smsd
}

// Selects messages from Outbox that have any recipient without sent flag set
const outboxGet = `SELECT
	o.id, o.src, o.report, o.body
FROM
	` + outboxTable + ` o
WHERE
	EXISTS (SELECT * FROM ` + recipientsTable + ` p WHERE p.msgId=o.id && !p.sent)
`

// Selects all recipients without sent flag set for givem msgId
const recipientsGet = `SELECT
	id, number
FROM
	` + recipientsTable + `
WHERE
	!sent && msgId=?
`

const recipientsSent = "UPDATE " + recipientsTable + " SET sent=? WHERE id=?"

// Send messages from Outbox
func (smsd *SMSd) sendMessages() (gammuError bool) {
	if !prepareOnce(smsd.db, &smsd.stmtOutboxGet, outboxGet) {
		return
	}
	if !prepareOnce(smsd.db, &smsd.stmtRecipGet, recipientsGet) {
		return
	}
	if !prepareOnce(smsd.db, &smsd.stmtRecipSent, recipientsSent) {
		return
	}
	msgs, res, err := smsd.stmtOutboxGet.Exec()
	if err != nil {
		log.Println("Can't get a messages from Outbox:", err)
		return
	}
	colMid := res.Map("id")
	colReport := res.Map("report")
	colBody := res.Map("body")
	for _, msg := range msgs {
		mid := msg.Uint(colMid)
		report := msg.Bool(colReport)
		body := msg.Str(colBody)

		recipients, res, err := smsd.stmtRecipGet.Exec(mid)
		if err != nil {
			log.Printf("Can't get a phone number for msg #%d: %s", mid, err)
			return
		}
		colPid := res.Map("id")
		colNum := res.Map("number")
		for _, p := range recipients {
			pid := p.Uint(colPid)
			num := p.Str(colNum)
			if !checkNumber(num) {
				continue
			}
			err = smsd.sm.SendLongSMS(num, body, report)
			if err != nil {
				if _, ok := err.(gammu.EncodeError); ok {
					log.Printf("Can't encode message to %s: %s", num, err)
					continue
				}
				smsd.gammuErrors++
				log.Printf("Can't send message to %s: %s", num, err)
				return true
			}
			_, _, err = smsd.stmtRecipSent.Exec(time.Now(), pid)
			if err != nil {
				log.Printf(
					"Can't mark a msg/recip #%d/#%d as sent: %s",
					mid, pid, err,
				)
				return
			}
		}
	}
	return
}

const inboxPut = `INSERT
	` + inboxTable + `
SET
	time=?,
	number=?,
	srcId=?,
	body=?,
	note=?
`

const recipReport = `UPDATE
	` + recipientsTable + `
SET
	report=?
WHERE
	!report && (number=? || concat(@localPrefix, number)=?)
ORDER BY
	abs(timediff(?, sent))
LIMIT 1`

type Msg struct {
	Time   time.Time
	Number string
	SrcId  uint
	Body   string
	Note   string
}

func (smsd *SMSd) recvMessages() (gammuError bool) {
	if !prepareOnce(smsd.db, &smsd.stmtInboxPut, inboxPut) {
		return
	}
	if !prepareOnce(smsd.db, &smsd.stmtRecipReport, recipReport) {
		return
	}
	if smsd.sqlNumToId != "" {
		if !prepareOnce(smsd.db, &smsd.stmtNumToId, smsd.sqlNumToId) {
			return
		}
	}

	var msg Msg
	smsd.stmtInboxPut.Bind(&msg)

	for {
		sms, err := smsd.sm.GetSMS()
		if err != nil {
			if err == io.EOF {
				break
			}
			smsd.gammuErrors++
			log.Printf("Can't get message from phone: %s", err)
			return true
		}
		if sms.Report {
			// Find a message and sender in Outbox and mark it
			m := strings.TrimSpace(sms.Body)
			if strings.ToLower(m) == "delivered" {
				_, _, err = smsd.stmtRecipReport.Exec(
					sms.SMSCTime, sms.Number, sms.Number, sms.Time,
				)
				if err != nil {
					log.Printf(
						"Can't mark recipient %s as reported: %s",
						sms.Number, err,
					)
					return
				}
			}
		} else {
			// Save a message in Inbox
			msg.Time = sms.Time
			msg.Number = sms.Number
			msg.SrcId = 0
			msg.Body = sms.Body
			//log.Printf("Odebrano: %+v", msg)
			if smsd.stmtNumToId.Raw != nil {
				id, _, err := smsd.stmtNumToId.ExecFirst(msg.Number)
				if err != nil {
					log.Printf(
						"Can't get srcId for number %s: %s",
						sms.Number, err,
					)
					return
				}
				if id != nil {
					msg.SrcId, err = id.UintErr(0)
					if err != nil {
						log.Printf("Bad srcId '%v': %s", id[0], err)
						return
					}
				}
			}
			if f := smsd.filter; f != nil {
				accept, err := f.Filter(&msg)
				if err != nil {
					log.Printf("Filter error: %s", err)
				} else if !accept {
					// Drop this message
					continue
				}
			}
			_, _, err = smsd.stmtInboxPut.Exec() // using msg
			if err != nil {
				log.Printf(
					"Can't insert message from %s into Inbox: %s",
					sms.Number, err,
				)
				return
			}
		}
	}
	return
}

const outboxDel = `DELETE FROM
	o
USING
	SMSd_Outbox o
WHERE
	o.del && !EXISTS(
		SELECT
			* 
		FROM
			SMSd_Recipients r
		WHERE
			r.msgId = o.id && (!r.sent || o.report && !r.report) 
	)
`

func (smsd *SMSd) delMessages() {
	if !prepareOnce(smsd.db, &smsd.stmtOutboxDel, outboxDel) {
		return
	}
	_, _, err := smsd.stmtOutboxDel.Exec()
	if err != nil {
		log.Println("Can't delete messages:", err)
		return
	}
}

func (smsd *SMSd) sendRecvDel(send bool) (end bool) {
	if smsd.gammuErrors > 2 && smsd.sm.IsConnected() {
		log.Println("Too many errors - disconnecting")
		smsd.sm.Disconnect()
	}
	if !smsd.sm.IsConnected() {
		if smsd.gammuConnErrors > 2 {
			log.Println("Too many connection errors - terminating")
			os.Exit(1)
		}
		err := smsd.sm.Connect()
		if err != nil {
			smsd.gammuConnErrors++
			sleep := 60 * time.Second
			log.Println("Can't connect:", err)
			log.Println("Waiting", sleep)
			select {
			case <-smsd.end:
				return true
			case <-time.After(sleep):
			}
			return
		}
		smsd.gammuErrors = 0
		smsd.gammuConnErrors = 0
	}

	if send {
		if smsd.sendMessages() {
			return
		}
	}
	if smsd.recvMessages() {
		return
	}
	if send {
		smsd.delMessages()
	}
	return
}

func (smsd *SMSd) loop() {
	send := true
	for {
		if smsd.sendRecvDel(send) {
			return
		}
		// Wait for some event or timeout
		select {
		case <-smsd.end:
			return
		case <-smsd.newMsg:
			send = true
		case <-time.After(smsd.pullInt):
			// if there is no newMsg signal, send and del two times less
			// frequently than recv
			send = !send
		}
	}
}

func (smsd *SMSd) Start() {
	go smsd.loop()
}

func (smsd *SMSd) Stop() {
	smsd.end <- event{}
}

func (smsd *SMSd) NewMsg() {
	select {
	case smsd.newMsg <- event{}:
	default:
	}
}
