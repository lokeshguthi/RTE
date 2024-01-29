package main

import (
	"io"
	"log"
)

var (
	Debug *log.Logger
	Info  *log.Logger
)

func InitLoggers(debugHandle io.Writer, infoHandle io.Writer) {
	Debug = log.New(debugHandle, "DEBUG: ", log.Ldate|log.Ltime)
	Info = log.New(infoHandle, "INFO: ", log.Ldate|log.Ltime)
}

func LogError(phase string, format string, values ...interface{}) {
	Info.Printf(format, values...)
	errorCounter.WithLabelValues(phase).Inc()
}
