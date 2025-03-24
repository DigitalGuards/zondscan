package logger

import (
	"os"
	"path/filepath"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func SyslogTimeEncoder(t time.Time, enc zapcore.PrimitiveArrayEncoder) {
	enc.AppendString(t.Format("Jan  2 15:04:05"))
}

func CustomLevelEncoder(level zapcore.Level, enc zapcore.PrimitiveArrayEncoder) {
	enc.AppendString("[" + level.CapitalString() + "]")
}

// FileLogger creates a configured zap logger that writes to both a file and stdout
func FileLogger(filename string) *zap.Logger {
	// Create logs directory if it doesn't exist
	logsDir := "logs"
	if err := os.MkdirAll(logsDir, 0755); err != nil {
		// If we can't create the logs directory, fall back to current directory
		logsDir = "."
	}

	// Use the logs directory for the log file
	fullpath := filepath.Join(logsDir, filename)

	// Create a production config with our customizations
	cfg := zap.NewProductionConfig()
	cfg.Encoding = "console"
	cfg.OutputPaths = []string{fullpath, "stdout"}
	cfg.ErrorOutputPaths = []string{fullpath, "stderr"}
	cfg.Level = zap.NewAtomicLevelAt(zapcore.DebugLevel)
	cfg.EncoderConfig.EncodeTime = SyslogTimeEncoder
	cfg.EncoderConfig.EncodeLevel = CustomLevelEncoder

	// Build the logger
	logger, err := cfg.Build()
	if err != nil {
		// If we can't build the logger, create a basic logger as fallback
		fallbackLogger, _ := zap.NewProduction()
		fallbackLogger.Error("Failed to build configured logger", zap.Error(err))
		return fallbackLogger
	}

	// Log the logger initialization
	logger.Info("Logger initialized",
		zap.String("file", fullpath),
		zap.String("level", cfg.Level.String()))

	return logger
}
