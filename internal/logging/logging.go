package logging

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
)

func New(localAppData string) (*log.Logger, func(), error) {
	dir := filepath.Join(localAppData, "VencordGuard", "logs")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, nil, err
	}
	file, err := os.OpenFile(filepath.Join(dir, "VencordGuard.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return nil, nil, err
	}
	logger := log.New(file, "", log.Ldate|log.Ltime|log.Lmicroseconds)
	return logger, func() { _ = file.Close() }, nil
}

func TailWriter(logger *log.Logger, prefix string) *writer {
	return &writer{logger: logger, prefix: prefix}
}

type writer struct {
	logger *log.Logger
	prefix string
}

func (w *writer) Write(p []byte) (int, error) {
	w.logger.Printf("%s %s", w.prefix, string(p))
	return len(p), nil
}

func Errorf(logger *log.Logger, format string, args ...any) error {
	err := fmt.Errorf(format, args...)
	logger.Printf("ERROR: %v", err)
	return err
}
