package main

import (
	"github.com/ziutek/gogammu"
	"github.com/ziutek/mymysql/autorc"
	_ "github.com/ziutek/mymysql/native"
	"io"
	"log"
	"strings"
	"time"
)

type SMSd struct {
	sm *gammu.StateMachine
	db *autorc.Conn

	end, newMsg chan event
	wait        bool

	stmtOutboxGet, stmtRecipGet, stmtRecipSent, stmtInboxPut,
	stmtRecipReport, stmtOutboxDel autorc.Stmt
}

func NewSMSd(cfg *Config) (*SMSd, error) {
	var err error
	smsd := new(SMSd)
	smsd.sm, err = gammu.NewStateMachine("")
	if err != nil {
		return nil, err
	}
	smsd.db = autorc.New(
		cfg.Db.Proto, cfg.Db.Saddr, cfg.Db.Daddr,
		cfg.Db.User, cfg.Db.Pass, cfg.Db.Name,
	)
	smsd.db.Raw.Register(setNames)
	smsd.db.Raw.Register(createOutbox)
	smsd.db.Raw.Register(createRecipients)
	smsd.db.Raw.Register(createInbox)
	smsd.db.Raw.Register(setLocPrefix)
	smsd.end = make(chan event)
	smsd.newMsg = make(chan event)
	return smsd, nil
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
func (smsd *SMSd) sendMessages() (gammuErr bool) {
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
			if isGammuError(smsd.sm.SendLongSMS(num, body, report)) {
				// Phone error or bad values
				gammuErr = true
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
	body=?
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

func (smsd *SMSd) recvMessages() (gammuErr bool) {
	if !prepareOnce(smsd.db, &smsd.stmtInboxPut, inboxPut) {
		return
	}
	if !prepareOnce(smsd.db, &smsd.stmtRecipReport, recipReport) {
		return
	}
	for {
		sms, err := smsd.sm.GetSMS()
		if err != nil {
			if err == io.EOF {
				break
			}
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
			_, _, err = smsd.stmtInboxPut.Exec(sms.Time, sms.Number, sms.Body)
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

func (smsd *SMSd) sendRecvDel(send bool) {
	var gammuErr bool
	if !smsd.sm.IsConnected() {
		if isGammuError(smsd.sm.Connect()) {
			return
		}
	}
	if send {
		gammuErr = smsd.sendMessages()
	}
	gammuErr = smsd.recvMessages() || gammuErr
	if send {
		smsd.delMessages()
	}
	if gammuErr && smsd.sm.IsConnected() {
		smsd.sm.Disconnect()
	}
}

func (smsd *SMSd) loop() {
	send := true
	for {
		smsd.sendRecvDel(send)
		// Wait for some event or timeout
		select {
		case <-smsd.end:
			return
		case <-smsd.newMsg:
			send = true
		case <-time.After(17 * time.Second):
			// send and del two times less frequently than recv
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
