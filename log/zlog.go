package log

import (
	"fmt"
	"github.com/rs/zerolog"
	"log"
	"os"
	"path"
	"time"
)

//var ZLog zerolog.Logger

type ZLogger struct {
	level      zerolog.Level
	//baseLogger zerolog.Logger
	baseFile   *os.File
}

type Message struct {
	event *zerolog.Event
	msg string
}

var ZLog zerolog.Logger
var gZLogger = ZLogger{}

var msgChan = make(chan Message, 10)
var QuitChan = make(chan int)

func ZLog_Init(strLevel string, pathname string, flag int)  {
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

	ZLog = zerolog.New(file).With().Caller().Timestamp().Logger().Output(zerolog.ConsoleWriter{
		Out:file,
		NoColor: true,
		TimeFormat: "2006/01/02 15:04:05",
	})
	gZLogger.level = level
	gZLogger.baseFile = file

	go func() {
		for {
			select {
			case msg := <- msgChan:
				msg.event.Msg(msg.msg)
			case <- QuitChan:
				return
			}
		}
	}()
}

func ZMsg(event *zerolog.Event, msg string) {
	//event.Msg(msg)
	msgChan <- Message{
		event: event,
		msg: msg,
	}
}
