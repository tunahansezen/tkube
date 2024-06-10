package core

import (
	"github.com/fatih/color"
	log "github.com/sirupsen/logrus"
	"io"
	logA "log"
)

var (
	Debug bool
	Trace bool
)

type PlainFormatter struct {
}

func (f *PlainFormatter) Format(entry *log.Entry) ([]byte, error) {
	if entry.Level == log.ErrorLevel {
		return []byte(color.RedString(entry.Message + "\n")), nil
	} else if entry.Level == log.TraceLevel {
		return []byte(color.YellowString(entry.Message + "\n")), nil
	}
	return []byte(color.CyanString(entry.Message + "\n")), nil
}

func toggleDebug() {
	logA.SetOutput(io.Discard)
	if Trace {
		log.SetLevel(log.TraceLevel)
	} else if Debug {
		log.SetLevel(log.DebugLevel)
	}
	log.SetFormatter(new(PlainFormatter))
}
