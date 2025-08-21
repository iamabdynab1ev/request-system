package logger

import "go.uber.org/zap"

type Logger struct{}

func NewLogger() *zap.Logger {
	dualConfig := zap.Config{
		Encoding:         "console",
		Level:            zap.NewAtomicLevelAt(zap.DebugLevel),
		OutputPaths:      []string{"stdout", "./logs/app.log"},
		ErrorOutputPaths: []string{"stderr"},
		EncoderConfig:    zap.NewProductionEncoderConfig(),
	}

	dualLogger, err := dualConfig.Build()
	if err != nil {
		panic(err)
	}

	return dualLogger
}
