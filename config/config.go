package config

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"

	"github.com/third-apps/go-zvec/types"
)

type LogConfig struct {
	Type        types.LogType
	Level       types.LogLevel
	Dir         string
	Basename    string
	FileSizeMB  uint32
	OverdueDays uint32
}

func NewConsoleLogConfig(level types.LogLevel) *LogConfig {
	return &LogConfig{Type: types.LogTypeConsole, Level: level}
}

func NewFileLogConfig(level types.LogLevel, dir, basename string, fileSizeMB, overdueDays uint32) *LogConfig {
	return &LogConfig{
		Type: types.LogTypeFile, Level: level,
		Dir: dir, Basename: basename,
		FileSizeMB: fileSizeMB, OverdueDays: overdueDays,
	}
}

type ConfigData struct {
	MemoryLimitBytes         uint64
	LogConfig                *LogConfig
	QueryThreadCount         uint32
	OptimizeThreadCount      uint32
	InvertToForwardScanRatio float32
	BruteForceByKeysRatio    float32
	FTSBruteForceByKeysRatio float32
	JiebaDictDir             string
}

type GlobalConfig struct {
	mu          sync.RWMutex
	data        ConfigData
	initialized bool
	logFile     *os.File
}

var globalConfig = &GlobalConfig{}

func Initialize(config *ConfigData) error {
	globalConfig.mu.Lock()
	defer globalConfig.mu.Unlock()

	if globalConfig.initialized {
		return errors.New("zvec is already initialized")
	}

	if config == nil {
		globalConfig.initialized = true
		return nil
	}

	globalConfig.data = *config

	if config.InvertToForwardScanRatio < 0 || config.InvertToForwardScanRatio > 1 {
		return fmt.Errorf("InvertToForwardScanRatio must be in [0,1], got %f", config.InvertToForwardScanRatio)
	}
	if config.BruteForceByKeysRatio < 0 || config.BruteForceByKeysRatio > 1 {
		return fmt.Errorf("BruteForceByKeysRatio must be in [0,1], got %f", config.BruteForceByKeysRatio)
	}
	if config.FTSBruteForceByKeysRatio < 0 || config.FTSBruteForceByKeysRatio > 1 {
		return fmt.Errorf("FTSBruteForceByKeysRatio must be in [0,1], got %f", config.FTSBruteForceByKeysRatio)
	}

	if config.LogConfig != nil {
		var level slog.Level
		switch config.LogConfig.Level {
		case types.LogLevelDebug:
			level = slog.LevelDebug
		case types.LogLevelInfo:
			level = slog.LevelInfo
		case types.LogLevelWarn:
			level = slog.LevelWarn
		case types.LogLevelError:
			level = slog.LevelError
		default:
			level = slog.LevelInfo
		}

		opts := &slog.HandlerOptions{Level: level}
		switch config.LogConfig.Type {
		case types.LogTypeFile:
			if config.LogConfig.Dir != "" {
				if err := os.MkdirAll(config.LogConfig.Dir, 0755); err != nil {
					return fmt.Errorf("failed to create log directory: %w", err)
				}
				logPath := filepath.Join(config.LogConfig.Dir, config.LogConfig.Basename+".log")
				f, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
				if err != nil {
					return fmt.Errorf("failed to open log file: %w", err)
				}
				globalConfig.logFile = f
				handler := slog.NewJSONHandler(f, opts)
				slog.SetDefault(slog.New(handler))
			}
		default:
			handler := slog.NewTextHandler(os.Stdout, opts)
			slog.SetDefault(slog.New(handler))
		}
	}

	globalConfig.initialized = true
	return nil
}

func Shutdown() {
	globalConfig.mu.Lock()
	defer globalConfig.mu.Unlock()
	if globalConfig.logFile != nil {
		globalConfig.logFile.Close()
		globalConfig.logFile = nil
	}
	globalConfig.initialized = false
}

func IsInitialized() bool {
	globalConfig.mu.RLock()
	defer globalConfig.mu.RUnlock()
	return globalConfig.initialized
}

func GetConfig() ConfigData {
	globalConfig.mu.RLock()
	defer globalConfig.mu.RUnlock()
	return globalConfig.data
}

func SetDefaultJiebaDictDir(dir string) {
	globalConfig.mu.Lock()
	defer globalConfig.mu.Unlock()
	globalConfig.data.JiebaDictDir = dir
}

func GetDefaultJiebaDictDir() string {
	globalConfig.mu.RLock()
	defer globalConfig.mu.RUnlock()
	return globalConfig.data.JiebaDictDir
}
