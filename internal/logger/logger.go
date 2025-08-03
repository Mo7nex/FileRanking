package logger

import (
	"io"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type Logger struct {
	infoLogger  *log.Logger
	warnLogger  *log.Logger
	errorLogger *log.Logger
	debugLogger *log.Logger
	file        *os.File
	mu          sync.Mutex
}

var (
	instance *Logger
	once     sync.Once
)

// GetInstance 获取单例日志器
func GetInstance() *Logger {
	once.Do(func() {
		instance = NewLogger()
	})
	return instance
}

// NewLogger 创建新的日志器
func NewLogger() *Logger {
	// 创建日志目录
	logDir := "logs"
	if err := os.MkdirAll(logDir, 0755); err != nil {
		log.Printf("创建日志目录失败: %v", err)
		return &Logger{
			infoLogger:  log.New(os.Stdout, "[INFO] ", log.LstdFlags),
			warnLogger:  log.New(os.Stdout, "[WARN] ", log.LstdFlags),
			errorLogger: log.New(os.Stderr, "[ERROR] ", log.LstdFlags),
			debugLogger: log.New(os.Stdout, "[DEBUG] ", log.LstdFlags),
		}
	}

	// 生成日志文件名
	logFile := filepath.Join(logDir, "app_"+time.Now().Format("2006-01-02")+".log")
	
	file, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		log.Printf("打开日志文件失败: %v", err)
		return &Logger{
			infoLogger:  log.New(os.Stdout, "[INFO] ", log.LstdFlags),
			warnLogger:  log.New(os.Stdout, "[WARN] ", log.LstdFlags),
			errorLogger: log.New(os.Stderr, "[ERROR] ", log.LstdFlags),
			debugLogger: log.New(os.Stdout, "[DEBUG] ", log.LstdFlags),
		}
	}

	// 创建多输出（文件+控制台）
	multiWriter := io.MultiWriter(os.Stdout, file)

	return &Logger{
		infoLogger:  log.New(multiWriter, "[INFO] ", log.LstdFlags|log.Lshortfile),
		warnLogger:  log.New(multiWriter, "[WARN] ", log.LstdFlags|log.Lshortfile),
		errorLogger: log.New(io.MultiWriter(os.Stderr, file), "[ERROR] ", log.LstdFlags|log.Lshortfile),
		debugLogger: log.New(multiWriter, "[DEBUG] ", log.LstdFlags|log.Lshortfile),
		file:        file,
	}
}

// Info 记录信息日志
func (l *Logger) Info(format string, v ...interface{}) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.infoLogger.Printf(format, v...)
}

// Warn 记录警告日志
func (l *Logger) Warn(format string, v ...interface{}) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.warnLogger.Printf(format, v...)
}

// Error 记录错误日志
func (l *Logger) Error(format string, v ...interface{}) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.errorLogger.Printf(format, v...)
}

// Debug 记录调试日志
func (l *Logger) Debug(format string, v ...interface{}) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.debugLogger.Printf(format, v...)
}

// Close 关闭日志文件
func (l *Logger) Close() error {
	if l.file != nil {
		return l.file.Close()
	}
	return nil
}

// Rotate 日志轮转
func (l *Logger) Rotate() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.file != nil {
		l.file.Close()
	}

	logDir := "logs"
	logFile := filepath.Join(logDir, "app_"+time.Now().Format("2006-01-02")+".log")
	
	file, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		return err
	}

	multiWriter := io.MultiWriter(os.Stdout, file)
	
	l.infoLogger.SetOutput(multiWriter)
	l.warnLogger.SetOutput(multiWriter)
	l.errorLogger.SetOutput(io.MultiWriter(os.Stderr, file))
	l.debugLogger.SetOutput(multiWriter)
	l.file = file

	return nil
}