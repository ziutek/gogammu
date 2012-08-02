package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"strings"
	"unicode"
)

type Config struct {
	Db      struct{ Proto, Saddr, Daddr, User, Pass, Name string }
	Listen  []string
	Source  []string
	LogFile string
	NumId   string
}

func syntaxError(ln int) error {
	return fmt.Errorf("syntax error at line: %d", ln)
}

func (c *Config) Read(r io.Reader) error {
	br := bufio.NewReader(r)
	for i := 1; ; i++ {
		buf, isPrefix, err := br.ReadLine()
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
		l := string(buf)
		if isPrefix {
			return errors.New("line too long")
		}
		l = strings.TrimFunc(l, unicode.IsSpace)
		if len(l) == 0 || l[0] == '#' {
			continue
		}
		n := strings.IndexFunc(l, unicode.IsSpace)
		if n == -1 {
			return syntaxError(i)
		}
		v := l[:n]
		l = strings.TrimLeftFunc(l[n:], unicode.IsSpace)
		switch v {
		case "DbSaddr":
			c.Db.Saddr = l
		case "DbDaddr":
			c.Db.Daddr = l
			c.Db.Proto = "tcp"
			if strings.IndexRune(l, ':') == -1 {
				c.Db.Proto = "unix"
			}
		case "DbUser":
			c.Db.User = l
		case "DbPass":
			c.Db.Pass = l
		case "DbName":
			c.Db.Name = l
		case "Source", "Listen":
			var a []string
			for {
				n := strings.IndexFunc(l, unicode.IsSpace)
				if n == -1 {
					a = append(a, l)
					break
				}
				a = append(a, l[:n])
				l = strings.TrimLeftFunc(l[n:], unicode.IsSpace)
			}
			switch v {
			case "Source":
				c.Source = a
			case "Listen":
				c.Listen = a
			}
		case "LogFile":
			c.LogFile = l
		case "NumId":
			c.NumId = l
		default:
			syntaxError(i)
		}

	}
	return nil
}
