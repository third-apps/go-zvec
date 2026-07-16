package config

import (
	"errors"
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
}

var globalConfig = &GlobalConfig{}

func Initialize(config *ConfigData) error {
	globalConfig.mu.Lock()
	defer globalConfig.mu.Unlock()

	if globalConfig.initialized {
		return errors.New("zvec is already initialized")
	}

	if config != nil {
		globalConfig.data = *config
	}
	globalConfig.initialized = true
	return nil
}

func Shutdown() {
	globalConfig.mu.Lock()
	defer globalConfig.mu.Unlock()
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
