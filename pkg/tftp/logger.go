package tftp

import (
	"fmt"
	"io"
	"log"
	"os"
)

type Logger struct {
	mainLogFile     *os.File
	requestsLogFile *os.File
}

func (l *Logger) Init(mainLogFileName string, requestsLogFile string) (err error) {
	// TODO: log rotation
	l.mainLogFile, err = os.OpenFile(mainLogFileName,
		os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0666)
	if err != nil {
		log.Printf("error opening file: %v", err)
	} else {
		// log to file AND to console :
		writer := io.MultiWriter(os.Stdout, l.mainLogFile)
		log.SetOutput(writer)
	}

	// TODO: log rotation
	l.requestsLogFile, err = os.OpenFile(requestsLogFile,
		os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0666)
	if err != nil {
		log.Printf("error opening file: %v", err)
	} else {
		// unlike l.mainLogFile, we do not write to both file and console because
		// requests log entries are already duplicated to l.mainLogFile: see LogRequest()
		writer := io.MultiWriter(os.Stdout, l.mainLogFile)
		log.SetOutput(writer)
	}
	return
}
func (l *Logger) DeInit() (err error) {
	if l.mainLogFile != nil {
		l.mainLogFile.Close()
		l.mainLogFile = nil
	}
	if l.requestsLogFile != nil {
		l.requestsLogFile.Close()
		l.requestsLogFile = nil
	}
	return
}
func (l *Logger) LogRequest(from string, request string, status string) {
	message := fmt.Sprintf("from=%v; request=%v; status=%v\n", from, request, status)
	if l.requestsLogFile != nil {
		l.requestsLogFile.Write([]byte(message))
	}
	log.Print("Request: " + message)
}
