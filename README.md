*gogammu* is binding for SMS related functions of *libGammu*.

[documentation](http://gopkgdoc.appspot.com/pkg/github.com/ziutek/gogammu)

*gogammu/smsd* is simple, MySQL based SMS daemon, written entirely in Go (it
doesn't depend on Gammu SMSd).

1. It receives messages from phone and save them in *Inbox*.
2. It waits for network connections (unix domain sockets to) and saves messages
in *Outbox*.
3. It sends messages from *Outbox*, waits for delivery reports and deletes
messages if necessary.

For run it in background use *runit* or *daemontools*.

*gogammu/sms* simple library that implements *smsd protocol*. Use it for sending
messages via *smsd*.

[documentation](http://gopkgdoc.appspot.com/pkg/github.com/ziutek/gogammu/sms)

Protocol description:

	FROM                                - symbol of source (<=16B)
	PHONE1[=DSTID1] PHONE2[=DSTID2] ... - list of phone numbers and dstIds
	Lines that contain optional parameters, one parameter per line: NAME or
	NAME VALUE. Implemented parameters:
	report        - report required
	delede        - delete message after sending (wait for reports, if required)
	              - empty line
	Message body
