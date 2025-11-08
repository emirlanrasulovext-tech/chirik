package observability

import (
	"os"
	"path/filepath"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func NewLogger(logFilePath string) (*zap.Logger, error) {
	config := zap.NewProductionConfig()
	config.EncoderConfig.TimeKey = "timestamp"
	config.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	config.EncoderConfig.MessageKey = "message"
	config.EncoderConfig.LevelKey = "level"
	config.EncoderConfig.CallerKey = "caller"

	if logFilePath != "" {
		dir := filepath.Dir(logFilePath)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, err
		}
		config.OutputPaths = []string{"stdout", logFilePath}
		config.ErrorOutputPaths = []string{"stderr", logFilePath}
	}

	logger, err := config.Build()
	if err != nil {
		return nil, err
	}

	return logger, nil
}
