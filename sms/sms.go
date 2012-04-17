package sms

import (
	"bufio"
	"net"
	"strings"
)

type Sender struct {
	Id     string // Server identifier. See Source field in smsd.cfg
	Server string // IP address:port or unix domain socket path
	Delete bool   // Will message need to be deleted after sent/reported?
	Report bool   // Is report required?
}

// Sends txt as SMS to recipents. Recipient need to be specified as
// PhoneNumber[=DstId] You can use DstId to link recipient with some other
// data in your database.
func (s *Sender) Send(txt string, recipients ...string) error {
	if len(recipients) == 0 {
		return nil
	}
	proto := "tcp"
	if strings.IndexRune(s.Server, ':') == -1 {
		proto = "unix"
	}
	c, err := net.Dial(proto, s.Server)
	if err != nil {
		return err
	}
	w := bufio.NewWriter(c)

	if err = writeln(w, s.Id); err != nil {
		return err
	}

	if _, err = w.WriteString(recipients[0]); err != nil {
		return err
	}
	for _, num := range recipients[1:] {
		if err = w.WriteByte(' '); err != nil {
			return err
		}
		if _, err = w.WriteString(num); err != nil {
			return err
		}
	}
	if err = newLine(w); err != nil {
		return err
	}

	if s.Delete {
		if err = writeln(w, "delete"); err != nil {
			return err
		}
	}
	if s.Report {
		if err = writeln(w, "report"); err != nil {
			return err
		}
	}

	if err = newLine(w); err != nil {
		return err
	}

	if _, err = w.WriteString(txt); err != nil {
		return err
	}

	if err = w.Flush(); err != nil {
		return err
	}
	return c.Close()
}

func newLine(w *bufio.Writer) error {
	return w.WriteByte('\n')
}

func writeln(w *bufio.Writer, s string) error {
	_, err := w.WriteString(s)
	if err != nil {
		return err
	}
	return newLine(w)
}
