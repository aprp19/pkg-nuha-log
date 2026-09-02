package logger

import (
	"os"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

// Init init zerolog config
func Init() {
	// Format waktu
	zerolog.TimeFieldFormat = time.RFC3339

	// Output ke console dengan style human friendly
	log.Logger = log.Output(zerolog.ConsoleWriter{
		Out:        os.Stderr,
		TimeFormat: "15:04:05",
	})
}

// Export log biar gampang dipakai di seluruh project
var (
	Info  = log.Info
	Error = log.Error
	Warn  = log.Warn
	Debug = log.Debug
	Fatal = log.Fatal
)
