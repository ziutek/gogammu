package main

import (
	"github.com/ziutek/gogammu"
	"github.com/ziutek/mymysql/mysql"
	_ "github.com/ziutek/mymysql/native"
)

type SMSd struct {
	db mysql.Conn
	sm *gammu.StateMachine
}

func NewSMSd(dbCfg DbCfg) (*SMSd, error) {
	var err error
	smsd := new(SMSd)
	smsd.sm, err = gammu.NewStateMachine("")
	if err != nil {
		return nil, err
	}
	smsd.db = mysql.New(dbCfg.Proto, dbCfg.Saddr, dbCfg.Daddr,
		dbCfg.User, dbCfg.Pass, dbCfg.Db,
	)
	return smsd, err
}

func (smsd *SMSd) Start() {

}

func (smsd *SMSd) Stop() {

}
