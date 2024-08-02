package util

import (
	"log"
	"log/slog"
	"os"
)

var Logger *slog.Logger

func init() {
	var programLevel = new(slog.LevelVar)
	h := slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: programLevel})
	Logger = slog.New(h)
	slog.SetDefault(Logger)
	programLevel.Set(slog.LevelDebug)

	logPath := "/var/log/system-usability-detection/system-usability-detection.log"
	file, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		log.Fatalf("无法打开日志文件: %v", err)
	}
	defer file.Close()
	log.SetOutput(file)
}
