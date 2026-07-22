package config

import (
	"testing"

	"github.com/third-apps/go-zvec/types"
)

// TestInitializeAndShutdown 验证全局配置初始化与关闭状态切换
func TestInitializeAndShutdown(t *testing.T) {
	Shutdown()
	err := Initialize(&ConfigData{})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if !IsInitialized() {
		t.Fatal("expected initialized")
	}
	Shutdown()
	if IsInitialized() {
		t.Fatal("expected not initialized after shutdown")
	}
}

// TestInitializeTwice 验证全局配置重复初始化返回错误
func TestInitializeTwice(t *testing.T) {
	Shutdown()
	err := Initialize(&ConfigData{})
	if err != nil {
		t.Fatalf("first init failed: %v", err)
	}
	err = Initialize(&ConfigData{})
	if err == nil {
		t.Fatal("expected error on double init")
	}
	Shutdown()
}

// TestGetConfig 验证全局配置读取正确性
func TestGetConfig(t *testing.T) {
	Shutdown()
	cfg := &ConfigData{
		MemoryLimitBytes: 1024 * 1024 * 512,
		QueryThreadCount: 4,
	}
	err := Initialize(cfg)
	if err != nil {
		t.Fatalf("init failed: %v", err)
	}
	got := GetConfig()
	if got.MemoryLimitBytes != cfg.MemoryLimitBytes {
		t.Fatalf("expected %d, got %d", cfg.MemoryLimitBytes, got.MemoryLimitBytes)
	}
	if got.QueryThreadCount != cfg.QueryThreadCount {
		t.Fatalf("expected %d, got %d", cfg.QueryThreadCount, got.QueryThreadCount)
	}
	Shutdown()
}

// TestJiebaDictDir 验证 Jieba 分词器字典目录的读取与设置
func TestJiebaDictDir(t *testing.T) {
	Shutdown()
	Initialize(&ConfigData{JiebaDictDir: "/tmp/dict"})
	dir := GetDefaultJiebaDictDir()
	if dir != "/tmp/dict" {
		t.Fatalf("expected '/tmp/dict', got '%s'", dir)
	}
	SetDefaultJiebaDictDir("/new/dict")
	dir = GetDefaultJiebaDictDir()
	if dir != "/new/dict" {
		t.Fatalf("expected '/new/dict', got '%s'", dir)
	}
	Shutdown()
}

// TestNewConsoleLogConfig 验证控制台日志配置构造
func TestNewConsoleLogConfig(t *testing.T) {
	cfg := NewConsoleLogConfig(types.LogLevelInfo)
	if cfg.Type != types.LogTypeConsole {
		t.Fatal("expected console log type")
	}
	if cfg.Level != types.LogLevelInfo {
		t.Fatal("expected info level")
	}
}

// TestNewFileLogConfig 验证文件日志配置构造
func TestNewFileLogConfig(t *testing.T) {
	cfg := NewFileLogConfig(types.LogLevelDebug, "/var/log", "zvec", 100, 7)
	if cfg.Type != types.LogTypeFile {
		t.Fatal("expected file log type")
	}
	if cfg.Dir != "/var/log" || cfg.Basename != "zvec" {
		t.Fatalf("unexpected dir/basename: %s/%s", cfg.Dir, cfg.Basename)
	}
	if cfg.FileSizeMB != 100 || cfg.OverdueDays != 7 {
		t.Fatalf("unexpected size/days: %d/%d", cfg.FileSizeMB, cfg.OverdueDays)
	}
}
