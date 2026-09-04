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

// Export log biar gampang dipakai di seluruh project.
// Error() auto-captures Msg + Str fields into the active access log request scope.
var (
	Info  = log.Info
	Warn  = log.Warn
	Debug = log.Debug
	Fatal = log.Fatal
)
