package log

import (
	"fmt"
	"github.com/rs/zerolog"
	"log"
	"os"
	"path"
	"runtime"
	"strings"
	"time"
)

type ZLogger struct {
	level      zerolog.Level
	baseFile   *os.File
}

type Message struct {
	event *zerolog.Event
	msg string
}

var ZLog zerolog.Logger
var gZLogger = ZLogger{}

var msgChan = make(chan Message, 100)
var QuitChan = make(chan int)

func ZLogInit(strLevel string, pathname string, _ int)  {
	file := getFilenameNow(pathname)
	var level zerolog.Level
	switch strLevel {
	case "debug":
		level = zerolog.DebugLevel
	case "release":
		level = zerolog.InfoLevel
	case "error":
		level = zerolog.ErrorLevel
	case "fatal":
		level = zerolog.FatalLevel
	default:
		log.Fatal("unknown level: " + strLevel)
	}
	zerolog.SetGlobalLevel(level)

	ZLog = zerolog.New(file).With().Timestamp().Logger().Output(zerolog.ConsoleWriter{
		Out:        file,
		NoColor:    true,
		TimeFormat: "2006/01/02 15:04:05",
	})
	gZLogger.level = level
	gZLogger.baseFile = file

	go func() {
		// 定时切换日志文件
		tick := time.Tick(12 * time.Hour)
		for {
			select {
			case <-tick:
				file := getFilenameNow(pathname)
				ZLog = zerolog.New(file).With().Timestamp().Logger().Output(zerolog.ConsoleWriter{
					Out:        file,
					NoColor:    true,
					TimeFormat: "2006/01/02 15:04:05",
				})
			case msg := <-msgChan:
				msg.event.Msg(msg.msg)
			case <-QuitChan:
				return
			}
		}
	}()
}

func getFilenameNow(pathname string) *os.File {
	now := time.Now()
	filename := fmt.Sprintf("%d%02d%02d_%02d_%02d_%02d_z.log",
		now.Year(),
		now.Month(),
		now.Day(),
		now.Hour(),
		now.Minute(),
		now.Second())
	file, err := os.Create(path.Join(pathname, filename))
	if err != nil {
		// Can we log an error before we have our logger? :)
		log.Fatal("create log file fail.")
	}
	return file
}

func ZeroLog(event *zerolog.Event, msg string) {
	//event.Msg(msg)
	_, file, line, ok := runtime.Caller(1)
	if !ok {
		Error("get caller fail:%v", msg)
		return
	}
	if idx := strings.LastIndex(file, "/"); idx != -1 {
		file = file[idx+1:]
	}
	// add caller.
	event = event.Str("caller", zerolog.CallerMarshalFunc(file, line))
	msgChan <- Message{
		event: event,
		msg: msg,
	}
}
