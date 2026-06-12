package logger

import (
	"os"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

var Log zerolog.Logger

func Init(env string) {
	zerolog.TimeFieldFormat = time.RFC3339

	if env == "development" {
		// Pretty console output for dev
		output := zerolog.ConsoleWriter{
			Out:        os.Stdout,
			TimeFormat: "15:04:05",
			NoColor:    false,
		}
		Log = zerolog.New(output).
			With().
			Timestamp().
			Caller().
			Logger()
	} else {
		// JSON output for production
		Log = zerolog.New(os.Stdout).
			With().
			Timestamp().
			Logger()
	}

	// Replace global logger
	log.Logger = Log
}

func WithRequestID(requestID string) zerolog.Logger {
	return Log.With().Str("request_id", requestID).Logger()
}

func WithUserID(userID string) zerolog.Logger {
	return Log.With().Str("user_id", userID).Logger()
}
