package main

const (
	setNames        = "SET NAMES utf8"
	setLocPrefix    = "SET @localPrefix='+48'"
	outboxTable     = "SMSd_Outbox"
	recipientsTable = "SMSd_Recipients"
	inboxTable      = "SMSd_Inbox"
)

const createOutbox = `CREATE TABLE IF NOT EXISTS ` + outboxTable + ` (
	id     int unsigned NOT NULL AUTO_INCREMENT,
	time   datetime NOT NULL,
	src    varchar(16) NOT NULL,
	report boolean NOT NULL,
	del    boolean NOT NULL,
	body   text NOT NULL,
	PRIMARY KEY (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8`

const createRecipients = `CREATE TABLE IF NOT EXISTS ` + recipientsTable + ` (
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

const createInbox = `CREATE TABLE IF NOT EXISTS ` + inboxTable + ` (
	id     int unsigned NOT NULL AUTO_INCREMENT,
	time   datetime NOT NULL,
	number varchar(16) NOT NULL,
	srcId  int unsigned NOT NULL,
	body   text NOT NULL,
	PRIMARY KEY (id),
	KEY srcId (srcId)
) ENGINE=MyISAM DEFAULT CHARSET=utf8`
