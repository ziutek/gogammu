package main

import (
	"github.com/ziutek/mymysql/autorc"
	_ "github.com/ziutek/mymysql/native"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
	"unicode"
)

var (
	ins         []*Input
	logFileName string
	smsd        *SMSd
)

func parseList(l string) []string {
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
	return a
}

func main() {
	if len(os.Args) != 2 {
		log.Printf("Usage: %s CONFIG_FILE\n", os.Args[0])
		os.Exit(1)
	}

	db, cfg, err := autorc.NewFromCF(os.Args[1])
	if err != nil {
		log.Println("Error:", err)
		os.Exit(1)
	}

	logFileName, _ = cfg["LogFile"]
	setupLogging()

	c, ok := cfg["Listen"]
	if !ok {
		log.Println("There is no 'Listen' option in config file")
		os.Exit(1)
	}
	listen := parseList(c)
	c, ok = cfg["Source"]
	if !ok {
		log.Println("There is no 'Source' option in config file")
		os.Exit(1)
	}
	source := parseList(c)

	pullInt := 17 * time.Second // if 15s my phone works bad
	c, _ = cfg["PullInt"]
	if c != "" {
		pullInt, err = time.ParseDuration(c)
		if err != nil {
			log.Printf("Wrong value for 'PullInt' option: '%s'", c)
			os.Exit(1)
		}
	}

	numId, _ := cfg["NumId"]
	filter, _ := cfg["Filter"]

	smsd = NewSMSd(db, numId, filter, pullInt)

	ins = make([]*Input, len(listen))
	for i, a := range listen {
		proto := "tcp"
		if strings.IndexRune(a, ':') == -1 {
			proto = "unix"
		}
		ins[i] = NewInput(smsd, proto, a, db.Clone(), source)
	}

	smsd.Start()
	defer smsd.Stop()

	for _, in := range ins {
		in.Start()
		defer in.Stop()
	}

	sc := make(chan os.Signal, 2)
	signal.Notify(sc, syscall.SIGTERM, syscall.SIGINT, syscall.SIGHUP)
	for sig := range sc {
		if sig == syscall.SIGHUP {
			setupLogging()
		} else {
			break
		}
	}
}

var logFile *os.File

func setupLogging() {
	if logFileName == "" {
		return
	}
	newFile, err := os.OpenFile(
		logFileName,
		os.O_WRONLY|os.O_APPEND|os.O_CREATE,
		0620,
	)
	if err != nil {
		log.Println(err)
		os.Exit(1)
	}
	prevFile := logFile
	logFile = newFile
	log.SetOutput(logFile)
	log.Println("Start logging to file:", logFileName)
	if prevFile != nil {
		err = prevFile.Close()
		if err != nil {
			log.Println(err)
			return
		}
	}
}
