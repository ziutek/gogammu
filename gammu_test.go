package gammu

import (
	"flag"
	"fmt"
	"os"
	"testing"
)

var number string

func init() {
	flag.StringVar(&number, "n", "", "phone number (required)")
	flag.Parse()
	if number == "" {
		flag.Usage()
		os.Exit(1)
	}
}

func checkErr(t *testing.T, e error) {
	if e != nil {
		t.Fatal(e)
	}
}

func TestSend(t *testing.T) {
	sm, err := NewStateMachine("")
	checkErr(t, err)
	checkErr(t, sm.Connect())
	checkErr(t, sm.SendSMS(number, "Test1 ąśćźż"))
	checkErr(t, sm.SendLongSMS(number, "Test2 'ąśćźż' The Go programming language is an open source project to make programmers more productive.  Go is expressive, concise, clean, and efficient. Its concurrency mechanisms make it easy to write programs that get the most out of multicore and networked machines, while its novel type system enables flexible and modular program construction."))
	checkErr(t, sm.Disconnect())
}

func TestGetNext(t *testing.T) {
	sm, err := NewStateMachine("")
	checkErr(t, err)
	checkErr(t, sm.Connect())
	sms, err := sm.GetNextSMS(true, true)
	checkErr(t, err)
	fmt.Println(sms)
}
