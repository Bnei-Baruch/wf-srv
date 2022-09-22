package api

import (
	"github.com/Bnei-Baruch/wf-srv/common"
	"io"
	"os"
	"path"

	"github.com/jasonlvhit/gocron"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"gopkg.in/natefinch/lumberjack.v2"
)

type Config struct {
	ConsoleLoggingEnabled bool
	EncodeLogsAsJson      bool
	FileLoggingEnabled    bool
	Directory             string
	Filename              string
	MaxSize               int
	MaxBackups            int
	MaxAge                int
	LocalTime             bool
	Compress              bool
}

func InitLog() {
	c := Config{
		ConsoleLoggingEnabled: false,
		FileLoggingEnabled:    true,
		EncodeLogsAsJson:      true,
		LocalTime:             true,
		Compress:              false,
		Directory:             common.LogPath,
		Filename:              "latest.log",
		MaxSize:               1000,
		MaxBackups:            0,
		MaxAge:                0,
	}

	var writers []io.Writer

	l := &lumberjack.Logger{
		Filename:   path.Join(c.Directory, c.Filename),
		MaxBackups: c.MaxBackups,
		MaxSize:    c.MaxSize,
		MaxAge:     c.MaxAge,
		LocalTime:  c.LocalTime,
		Compress:   c.Compress,
	}

	if err := os.MkdirAll(c.Directory, 0744); err != nil {
		log.Error().Err(err).Str("path", c.Directory).Msg("can't create log directory")
	}

	if c.ConsoleLoggingEnabled {
		writers = append(writers, zerolog.ConsoleWriter{Out: os.Stderr})
	}
	if c.FileLoggingEnabled {
		writers = append(writers, l)
	}
	mw := io.MultiWriter(writers...)

	if common.DEBUG == "1" {
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	} else {
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
	}
	zerolog.MessageFieldName = "msg"
	log.Logger = zerolog.New(mw).With().Timestamp().Logger()

	gocron.Every(1).Day().At("23:59:59").Do(l.Rotate)
	gocron.Start()
}
