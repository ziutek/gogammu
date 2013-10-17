package sms

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"net"
	"strings"
)

type Sender struct {
	Id     string // Sender identifier. See Source field in smsd.cfg
	Server string // IP address:port or unix domain socket path
	Delete bool   // Will message need to be deleted after sent/reported?
	Report bool   // Is report required?
}

// Sends txt as SMS to recipients. Recipient need to be specified as
// PhoneNumber[=DstId] You can use DstId to link recipient with some other
// data in your database. Send is thread-safe.
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
	defer c.Close()

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

	if _, err = w.WriteString(strings.TrimSpace(txt)); err != nil {
		return err
	}
	if err = writeln(w, "\n."); err != nil {
		return err
	}
	if err = w.Flush(); err != nil {
		return err
	}
	// Read OK response
	buf, _, err := bufio.NewReader(c).ReadLine()
	if err != nil {
		return err
	}
	buf = bytes.TrimSpace(buf)
	if !bytes.Equal(buf, []byte{'O', 'K'}) {
		return errors.New(string(buf))
	}
	return nil
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

// Returns number of characters that will be used to send txt via SMS
func Len(txt string) int {
	m, n := 1, 0
	for _, r := range txt {
		if r > 0x7F {
			m = 4
		}
		n++
	}
	return m * n
}

func AppendId(phones []string, id int) []string {
	s := fmt.Sprintf("=%d", id)
	r := make([]string, len(phones))
	for i, p := range phones {
		r[i] = p + s
	}
	return r
}
