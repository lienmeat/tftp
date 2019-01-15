package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"runtime/trace"
	"time"

	"github.com/lienmeat/tftp"
	"github.com/lienmeat/tftp/udpserver"
	log "github.com/sirupsen/logrus"
)

func main() {
	address := flag.String("address", "", "IP address and port to use (ex: 0.0.0.0:69)")
	logLevel := flag.String("logLevel", "info", "logging level (trace, debug, info, warn, error, panic, fatal)")
	logFile := flag.String("logFile", "", "log file, if not set, will log to stdOut")
	requestLogFile := flag.String("requestsLogFile", "tftp_requests.log", "requests log file, if not set, will log to tftp_requests.log")
	traceFile := flag.String("traceFile", "", "trace execution to file")
	flag.Parse()

	if *address == "" {
		flag.Usage()
		return
	}

	ctx, done := context.WithCancel(context.Background())

	if *traceFile != "" {
		//do a 90 second trace
		go func() {
			<-time.After(time.Second * 90)
			done()
		}()
		tfh, err := os.OpenFile(*traceFile, os.O_WRONLY|os.O_CREATE, 0775)
		if err != nil {
			panic("err opening trace file: " + err.Error())
		}
		trace.Start(tfh)
		defer trace.Stop()
	}

	if lvl, err := log.ParseLevel(*logLevel); err == nil {
		log.SetFormatter(&log.JSONFormatter{})
		log.SetLevel(lvl)
	} else {
		log.SetLevel(log.InfoLevel)
	}

	if *logFile != "" {
		if lfh, err := os.OpenFile(*logFile, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0775); err == nil {
			defer lfh.Close()
			log.SetOutput(lfh)
		} else {
			panic("err opening log file: " + err.Error())
		}
	}

	if *requestLogFile == "" {
		*requestLogFile = "tftp_requests.log"
	}
	rlf, err := tftp.SetupRequestLog(*requestLogFile)
	if err != nil {
		panic(fmt.Sprintf("couldn't open %s: %s", *requestLogFile, err.Error()))
	}
	defer rlf.Close()

	if err := udpserver.Server(ctx, *address, tftp.NewTFTPProtocolHandler()); err != nil {
		panic(err)
	}
}
