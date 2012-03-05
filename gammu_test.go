package gammu

import (
	"flag"
	"fmt"
	"io"
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
	checkErr(t, sm.SendSMS(number, "Test1 ąśćźż", true))
	checkErr(t, sm.SendLongSMS(number, "Test2 'ąśćźż' The Go programming language is an open source project to make programmers more productive.  Go is expressive, concise, clean, and efficient.", true))
	checkErr(t, sm.Disconnect())
}

func TestGet(t *testing.T) {
	sm, err := NewStateMachine("")
	checkErr(t, err)
	checkErr(t, sm.Connect())
	for {
		sms, err := sm.GetSMS()
		if err == io.EOF {
			break
		}
		checkErr(t, err)
		fmt.Printf("received SMS: %+v\n", sms)
	}
	checkErr(t, sm.Disconnect())
}
