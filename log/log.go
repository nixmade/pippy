package log

import (
	"os"
	"path"

	"github.com/rs/zerolog"
	"gopkg.in/natefinch/lumberjack.v2"
)

var (
	DefaultLogger *zerolog.Logger = nil
)

func Get() *zerolog.Logger {
	if DefaultLogger != nil {
		return DefaultLogger
	}

	homedir, err := os.UserHomeDir()
	if err != nil {
		panic(err)
	}

	newLogger := zerolog.New(&lumberjack.Logger{
		Filename:   path.Join(homedir, ".pippy", "logs", "pippy.log"),
		MaxBackups: 3,  // files
		MaxSize:    10, // megabytes
		MaxAge:     7,  // days
	}).With().
		Caller().
		Timestamp().
		Logger().
		Level(zerolog.DebugLevel)

	DefaultLogger = &newLogger

	return DefaultLogger
}
