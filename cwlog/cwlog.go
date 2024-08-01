package cwlog

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"time"
)

var (
	logDesc  *os.File
	logName  string
	logReady bool

	logBuf      []string
	logBufLines int
	logBufLock  sync.Mutex
)

/*
 * Log this, can use printf arguments
 * Write to buffer, async write
 */
func DoLog(withTrace bool, format string, args ...interface{}) {
	var buf string

	if withTrace {
		/* Get current time */
		ctime := time.Now()
		/* Get calling function and line */
		_, filename, line, _ := runtime.Caller(1)
		/* printf conversion */
		text := fmt.Sprintf(format, args...)
		/* Add current date */
		date := fmt.Sprintf("%2v:%2v.%2v", ctime.Hour(), ctime.Minute(), ctime.Second())
		/* Date, go file, go file line, text */
		buf = fmt.Sprintf("%v: %15v:%5v: %v\n", date, filepath.Base(filename), line, text)
	} else {
		/* Get current time */
		ctime := time.Now()
		/* printf conversion */
		text := fmt.Sprintf(format, args...)
		/* Add current date */
		date := fmt.Sprintf("%2v:%2v.%2v", ctime.Hour(), ctime.Minute(), ctime.Second())
		/* Date, go file, go file line, text */
		buf = fmt.Sprintf("%v: %v\n", date, text)
	}

	if !logReady || logDesc == nil {
		fmt.Print(buf)
		return
	}

	/* Add to buffer */
	logBufLock.Lock()
	logBuf = append(logBuf, buf)
	logBufLines++
	logBufLock.Unlock()
}

func LogDaemon() {

	go func() {
		for {
			logBufLock.Lock()

			/* Are there lines to write? */
			if logBufLines == 0 {
				logBufLock.Unlock()
				time.Sleep(time.Millisecond * 100)
				continue
			}

			/* Write line */
			_, err := logDesc.WriteString(logBuf[0])
			if err != nil {
				fmt.Println("DoLog: WriteString failure")
				logDesc.Close()
				logDesc = nil
			}
			fmt.Print(logBuf[0])

			/* Remove line from buffer */
			logBuf = logBuf[1:]
			logBufLines--

			logBufLock.Unlock()
		}
	}()
}

/* Prep logger */
func StartLog() {
	t := time.Now()

	/* Create our log file names */
	logName = fmt.Sprintf("log/auth-%v-%v-%v.log", t.Day(), t.Month(), t.Year())

	/* Make log directory */
	errr := os.MkdirAll("log", os.ModePerm)
	if errr != nil {
		fmt.Print(errr.Error())
		return
	}

	/* Open log files */
	bdesc, errb := os.OpenFile(logName, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)

	/* Handle file errors */
	if errb != nil {
		fmt.Printf("An error occurred when attempting to create the log. Details: %s", errb)
		return
	}

	/* Save descriptors, open/closed elsewhere */
	logDesc = bdesc
	logReady = true

}
