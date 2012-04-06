package main

import (
	"github.com/ziutek/gogammu"
	"github.com/ziutek/mymysql/autorc"
	_ "github.com/ziutek/mymysql/native"
	"log"
	"time"
)

type SMSd struct {
	db *autorc.Conn
	sm *gammu.StateMachine
}

func NewSMSd(dbCfg DbCfg) (*SMSd, error) {
	var err error
	smsd := new(SMSd)
	smsd.sm, err = gammu.NewStateMachine("")
	if err != nil {
		return nil, err
	}
	smsd.db = autorc.New(
		dbCfg.Proto, dbCfg.Saddr, dbCfg.Daddr,
		dbCfg.User, dbCfg.Pass, dbCfg.Db,
	)
	smsd.db.Raw.Register(setNames)
	smsd.db.Raw.Register(createOutbox)
	return smsd, nil
}

const outboxGet = `SELECT
	o.id, o.src, o.body
FROM
	` + outboxTable + ` o
WHERE
	EXISTS (SELECT * FROM ` + phonesTable + ` p WHERE p.msgId=o.id && !p.sent)
`

const phonesGet = `SELECT
	id, number
FROM
	` + phonesTable + `
WHERE
	!sent && msgId=?
`

const (
	phonesSent = "UPDATE " + phonesTable + " SET sent=? WHERE id=?"
	outboxDel  = "DELETE FROM " + outboxTable + " WHERE id=?"
)

func (smsd *SMSd) loop() {
	var stmtOutboxGet, stmtOutboxDel, stmtPhonesGet, stmtPhonesSent autorc.Stmt
	for {
		time.Sleep(5 * time.Second)
		if !prepareOnce(smsd.db, &stmtOutboxGet, outboxGet) {
			continue
		}
		if !prepareOnce(smsd.db, &stmtOutboxDel, outboxDel) {
			continue
		}
		if !prepareOnce(smsd.db, &stmtPhonesGet, phonesGet) {
			continue
		}
		if !prepareOnce(smsd.db, &stmtPhonesSent, phonesSent) {
			continue
		}
		msgs, res, err := stmtOutboxGet.Exec()
		if err != nil {
			log.Println("Can't get a messages from Outbox:", err)
			continue
		}
		if len(msgs) == 0 {
			continue
		}
		if isGammuError(smsd.sm.Connect()) {
			continue
		}
		colMid := res.Map("id")
		colSrc := res.Map("src")
		colBody := res.Map("body")
		for _, msg := range msgs {
			mid := msg.Uint(colMid)
			src := msg.Str(colSrc)
			body := msg.Str(colBody)

			log.Printf("#%d src=%s body='%s'", mid, src, body)

			phones, res, err := stmtPhonesGet.Exec(mid)
			if err != nil {
				log.Printf("Can't get a phone number for msg #%d: %s", mid, err)
				continue
			}
			colPid := res.Map("id")
			colNum := res.Map("number")
			for _, p := range phones {
				pid := p.Uint(colPid)
				num := p.Str(colNum)
				if isGammuError(smsd.sm.SendLongSMS(num, body, false)) {
					continue
				}
				_, _, err = stmtPhonesSent.Exec(time.Now(), pid)
				if err != nil {
					log.Printf(
						"Can't mark a msg/phone #%d/#%d as sent: %s",
						mid, pid, err,
					)
					continue
				}
			}
		}
		if isGammuError(smsd.sm.Disconnect()) {
			continue
		}
	}
}

func (smsd *SMSd) Start() {
	go smsd.loop()
}

func (smsd *SMSd) Stop() {

}
