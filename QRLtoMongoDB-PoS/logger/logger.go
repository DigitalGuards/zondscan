package logger

import (
	"path/filepath"
	"time"
	"os"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func SyslogTimeEncoder(t time.Time, enc zapcore.PrimitiveArrayEncoder) {
	enc.AppendString(t.Format("Jan  2 15:04:05"))
}

func CustomLevelEncoder(level zapcore.Level, enc zapcore.PrimitiveArrayEncoder) {
	enc.AppendString("[" + level.CapitalString() + "]")
}

func FileLogger(filename string) *zap.Logger {
	path, err := os.Getwd()
	if err != nil {
		os.Exit(0)
	}

	fullpath := filepath.Join(path, "/logs.log")

	cfg := zap.NewProductionConfig()

	cfg.Encoding = "console"
	cfg.OutputPaths = []string{fullpath}
	cfg.Level = zap.NewAtomicLevelAt(zapcore.DebugLevel)
	cfg.EncoderConfig.EncodeTime = SyslogTimeEncoder
	cfg.EncoderConfig.EncodeLevel = CustomLevelEncoder

	logger, _ := cfg.Build()

	return logger
}
