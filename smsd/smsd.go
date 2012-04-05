package main

import (
	"github.com/ziutek/gogammu"
	"github.com/ziutek/mymysql/autorc"
	_ "github.com/ziutek/mymysql/native"
	"log"
	"strings"
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

const outboxGetAll = "select * from " + outboxTable
const outboxDel = "delete from " + outboxTable + " where id=?"

func (smsd *SMSd) loop() {
	var get, del autorc.Stmt
	for {
		time.Sleep(5 * time.Second)
		if !prepareOnce(smsd.db, &get, outboxGetAll) {
			continue
		}
		if !prepareOnce(smsd.db, &del, outboxDel) {
			continue
		}
		rows, res, err := get.Exec()
		if err != nil {
			log.Println("Can't get messages from Outbox:", err)
			continue
		}
		if len(rows) == 0 {
			continue
		}
		if isGammuError(smsd.sm.Connect()) {
			continue
		}
		id := res.Map("id")
		src := res.Map("src")
		dst := res.Map("dst")
		body := res.Map("body")

	msgLoop:
		for _, row := range rows {
			i := row.Uint(id)
			from := row.Str(src)
			to := strings.Split(row.Str(dst), " ")
			txt := row.Str(body)

			log.Printf("#%d from=%s to=%v txt='%s'", i, from, to, txt)
			for _, number := range to {
				if isGammuError(smsd.sm.SendLongSMS(number, txt, false)) {
					continue msgLoop
				}
			}

			if _, _, err := del.Exec(i); err != nil {
				log.Printf("Can't delete message #%d from Outbox", i)
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
