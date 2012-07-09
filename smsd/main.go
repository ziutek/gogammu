package main

import (
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
)

var (
	cfg  Config
	ins  []*Input
	smsd *SMSd
)

func main() {
	if len(os.Args) != 2 {
		log.Printf("Usage: %s CONFIG_FILE\n", os.Args[0])
		os.Exit(1)
	}
	cf, err := os.Open(os.Args[1])
	if err != nil {
		log.Println("Can't open configuration file:", err)
		os.Exit(1)
	}
	err = cfg.Read(cf)
	if err != nil {
		log.Println("Can't read configuration file:", err)
		os.Exit(1)
	}

	setupLogging(cfg.Logging)

	smsd, err = NewSMSd(&cfg)
	if err != nil {
		log.Println("Error:", err)
		os.Exit(1)
	}

	ins = make([]*Input, len(cfg.Listen))
	for i, a := range cfg.Listen {
		proto := "tcp"
		if strings.IndexRune(a, ':') == -1 {
			proto = "unix"
		}
		ins[i] = NewInput(smsd, proto, a, &cfg)
	}

	smsd.Start()
	defer smsd.Stop()

	for _, in := range ins {
		if err := in.Start(); err != nil {
			log.Print("Can't start input thread: ", in)
			return
		}
		defer in.Stop()
	}

	sc := make(chan os.Signal, 2)
	signal.Notify(sc, syscall.SIGTERM, syscall.SIGINT, syscall.SIGHUP)
	for sig := range sc {
		if sig == syscall.SIGHUP {
			setupLogging(cfg.Logging)
		} else {
			break
		}
	}
}

var logFile *os.File

func setupLogging(fname string) {
	if fname == "" {
		return
	}
	newFile, err := os.Create(fname)
	if err != nil {
		log.Println(err)
		return
	}
	prevFile := logFile
	logFile = newFile
	log.SetOutput(logFile)
	log.Println("Start logging to file:", fname)
	if prevFile != nil {
		err = prevFile.Close()
		if err != nil {
			log.Println(err)
			return
		}
	}
}
