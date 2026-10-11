package logging

import (
	"io"
	"log"
	"log/slog"
	"os"
	"strings"
)

// Opts configures the custom structured logger.
type Opts struct {
	w     io.Writer
	level slog.Level
}

// NewOpts creates options for a new structured logger. It takes the logFile and
// level as strings, tests if logFile exists and can be written to, exiting if not,
// defaulting to os.Stdout, and attempts to map level to a slog.Level, defaulting
// to INFO.
func NewOpts(logFile, level string) Opts {
	var w io.Writer
	if logFile != "" {
		f, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
		if err != nil {
			log.Fatalf("error creating logger: cannot open log file: %v", err)
		}
		w = f
	} else {
		w = os.Stdout
		logFile = "stdout"
	}

	level = strings.ToLower(level)
	var l slog.Level
	var ok bool
	if l, ok = levelMap[level]; !ok {
		log.Printf("WARN: configured log level '%s' is invalid. Defaulting to INFO...", level)
		l = slog.LevelInfo
	}

	log.Printf("configured logger: level=%v, path=%s", level, logFile)
	return Opts{w: w, level: l}
}

var levelMap = map[string]slog.Level{
	"debug": slog.LevelDebug,
	"info":  slog.LevelInfo,
	"warn":  slog.LevelWarn,
	"error": slog.LevelError,
}

// New creates a new structured logger.
func New(opts Opts) *slog.Logger {
	t := slog.NewTextHandler(opts.w, &slog.HandlerOptions{
		Level: opts.level,
	})
	return slog.New(t)
}
