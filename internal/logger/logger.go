// Package logger 负责日志收敛，给日志添加前缀和独立文件输出，以便在本框架被集成之后的日志可读
package logger

import (
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// 日志级别定义
const (
	TraceLevel    = 6 // 可详细跟踪
	DebugLevel    = 5 // 可调式
	InfoLevel     = 4 // 日常使用关键信息
	WarningLevel  = 3 // 警告级别信息
	ErrorLevel    = 2 // 错误级别信息
	CriticalLevel = 1 // 严重错误
)

const (
	defaultPrefix = "[goom-mocker]" // defaultPrefix 默认日志前缀
	openDebugEnv  = "GOOM_DEBUG"    // openDebugEnv 开启 debug 日志
)

var (
	// LogLevel 日志级别
	// level 总共分5个级别：debug < info< warning< error< critical
	LogLevel = InfoLevel
	// ConsoleLevel 控制台打印级别
	ConsoleLevel = WarningLevel
	// ShowError2Console 把错误同步打印到控制台
	ShowError2Console = false
	// Logger 独立日志文件
	Logger io.Writer = os.Stdout
	// EnableLogTrack 开启并发日志染色
	EnableLogTrack = false
	// trackGetter 日志染色器, 用于并发测试区分协程 ID
	trackGetter func() string
	// logFile 日志路径
	logFile *os.File
)

var (
	levelName = map[int]string{ // levelName 日志级别-级别名称映射
		TraceLevel:    "trace",
		DebugLevel:    "debug",
		InfoLevel:     "info",
		WarningLevel:  "warn",
		ErrorLevel:    "error",
		CriticalLevel: "critical",
	}
	levelColor = map[int]Color{ // levelColor 日志级别-颜色映射
		TraceLevel:    None,
		DebugLevel:    None,
		InfoLevel:     None,
		WarningLevel:  Yellow,
		ErrorLevel:    Red,
		CriticalLevel: Red,
	}
)

// init 初始化
func init() {
	if d := os.Getenv(openDebugEnv); d != "" {
		OpenDebug()
	}

	loggerDir, err := loggerPath()
	if err != nil {
		fmt.Println("loggerDir error:", err)
		return
	}

	loggerPath := filepath.Join(loggerDir, "goom-mocker.log")
	logFile, err = os.OpenFile(loggerPath, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0766)
	if err != nil {
		fmt.Println("init log file error:", err)
		return
	}
	Logger = logFile
	fmt.Println("goom-mocker logFileLocation:", loggerPath)
}

// OpenDebug 开启 debug 模式
func OpenDebug() {
	ConsoleLevel = DebugLevel
}

// CloseDebug 关闭 debug 模式
func CloseDebug() {
	ConsoleLevel = WarningLevel
}

// IsDebugOpen 是否开启 debug 模式
func IsDebugOpen() bool {
	return ConsoleLevel >= DebugLevel
}

// OpenTrace 打开日志跟踪
func OpenTrace() {
	OpenDebug()
	SetLog2Console(true)
	LogLevel = TraceLevel
}

// CloseTrace 关闭日志跟踪
func CloseTrace() {
	CloseDebug()
	LogLevel = InfoLevel
	SetLog2Console(false)
}

// SetLog2Console 设置是否打印到控制台
func SetLog2Console(b bool) {
	if b {
		Logger = os.Stdout
	} else {
		Logger = logFile
	}
}

// SetLogTrack 设置日志染色
func SetLogTrack(enable bool, getter func() string) {
	if enable && getter != nil {
		trackGetter = getter
		EnableLogTrack = true
	} else {
		EnableLogTrack = false
	}
}

// TraceEnable 是否开启 trace
func TraceEnable() bool {
	return LogLevel >= TraceLevel
}

// Trace 打印 trace 日志
func Trace(v ...interface{}) {
	if LogLevel >= TraceLevel {
		_, _ = Logger.Write(layout(TraceLevel, v))
	}
}

// Tracef 打印 trace 日志
func Tracef(format string, a ...interface{}) {
	if LogLevel >= TraceLevel {
		_, _ = Logger.Write(layoutf(TraceLevel, format, nil, a...))
	}
}

// DebugEnable 是否开启 debug
func DebugEnable() bool {
	return LogLevel >= DebugLevel
}

// Debug 打印 debug 日志
func Debug(v ...interface{}) {
	if LogLevel >= DebugLevel {
		_, _ = Logger.Write(layout(DebugLevel, v))
	}
}

// Debugf 打印 debug 日志
func Debugf(format string, a ...interface{}) {
	if LogLevel >= DebugLevel {
		_, _ = Logger.Write(layoutf(DebugLevel, format, nil, a...))
	}
}

// Info 打印 info 日志
func Info(v ...interface{}) {
	if LogLevel >= InfoLevel {
		_, _ = Logger.Write(layout(InfoLevel, v))
	}
}

// Infof 打印 info 日志
func Infof(format string, a ...interface{}) {
	if LogLevel >= InfoLevel {
		_, _ = Logger.Write(layoutf(InfoLevel, format, nil, a...))
	}
}

// Warning 打印 warning 日志
func Warning(v ...interface{}) {
	if LogLevel >= WarningLevel {
		line := layout(WarningLevel, v)
		_, _ = Logger.Write(line)
		write2Console(line)
	}
}

// Warningf 打印 warning 日志
func Warningf(format string, a ...interface{}) {
	if LogLevel >= WarningLevel {
		line := layoutf(WarningLevel, format, nil, a...)
		_, _ = Logger.Write(line)
		write2Console(line)
	}
}

// Important 打印重要的日志
func Important(v ...interface{}) {
	line := layout(InfoLevel, v)
	_, _ = Logger.Write(line)
	write2Console(line)
}

// Importantf 打印重要的日志
func Importantf(format string, a ...interface{}) {
	line := layoutf(InfoLevel, format, nil, a...)
	_, _ = Logger.Write(line)
	write2Console(line)
}

// Error 打印 error 日志
func Error(v ...interface{}) {
	if LogLevel >= ErrorLevel {
		line := layout(ErrorLevel, v)
		_, _ = Logger.Write(line)
		write2Console(line)
	}
}

// Errorf 打印 error 日志
func Errorf(format string, a ...interface{}) {
	if LogLevel >= ErrorLevel {
		line := layoutf(ErrorLevel, format, nil, a...)
		_, _ = Logger.Write(line)
		write2Console(line)
	}
}

// Console 打印日志到控制台
func Console(level int, s string) {
	if level <= ConsoleLevel {
		os.Stdout.Write([]byte(s))
	}
}

// Consolef 打印日志到控制台
func Consolef(level int, format string, a ...interface{}) {
	if level <= ConsoleLevel {
		line := layoutf(level, format, nil, a...)
		os.Stdout.Write(line)
	}
}

// Consolefc 打印日志到控制台
func Consolefc(level int, format string, callerFn CallerFn, a ...interface{}) {
	if level <= ConsoleLevel {
		line := layoutf(level, format, callerFn, a...)
		os.Stdout.Write(line)
	}
}

// write2Console 输出到控制台，如果 ShowError2Console==true 时
func write2Console(line []byte) {
	if ShowError2Console {
		os.Stdout.Write(line)
	}
}

// layout 给日志格式化，给日志带上[goom]前缀, 方便与业务日志区分
func layout(level int, v []interface{}) []byte {
	arr := make([]string, 0, len(v)+1)
	arr = append(arr, time.Now().Format("2006-01-02 15:04:05"))
	arr = append(arr, defaultPrefix, "["+levelName[level]+"]:")

	if EnableLogTrack {
		arr = append(arr, trackGetter())
	}

	for _, a := range v {
		arr = append(arr, fmt.Sprintf("%s", a))
	}

	arr = append(arr, "\n")
	line := strings.Join(arr, " ")
	line = levelColor[level].Add(line)
	return []byte(line)
}

// layoutf 给日志格式化，带上[goom]前缀和添加颜色等
func layoutf(level int, format string, callerFn CallerFn, a ...interface{}) []byte {
	time := time.Now().Format("2006-01-02 15:04:05")
	if EnableLogTrack {
		return []byte(time + " " + defaultPrefix + "[" + levelName[level] + "]: " +
			trackGetter() + " " + fmt.Sprintf(format, a...) + "\n")
	}

	line := time + " " + defaultPrefix + "[" + levelName[level] + "]: "
	if callerFn != nil {
		line += callerFn() + " "
	}
	line += fmt.Sprintf(format, a...) + "\n"
	line = levelColor[level].Add(line)
	return []byte(line)
}

// loggerPath 获取日志存储路径
func loggerPath() (string, error) {
	var logFileLocation = "."
	// 获取当前目录
	dir, err := filepath.Abs(filepath.Dir(os.Args[0]))
	homeDir, err := Dir()
	if err == nil {
		dir = homeDir
	}
	if err == nil && dir != "/" {
		logFileLocation = filepath.Join(dir, "logs")
	}

	// 判断文件夹是否存在
	_, err = os.Stat(logFileLocation)
	if err == nil {
		return logFileLocation, err
	}
	if os.IsNotExist(err) {
		err = os.Mkdir(logFileLocation, os.ModePerm)
		if err != nil {
			fmt.Println("init log file error:", err)
			return ".", err
		}
	}
	return ".", err
}

// CallerFn 获取 Caller 行号的回调函数类型
type CallerFn func() string

// Caller 默认的 CallerFn, 用于 debug 日志获取调用者的行号
func Caller(skip int) func() string {
	return func() string {
		return caller(skip)
	}
}

func caller(skip int) string {
	frame, defined := getCallerFrame(skip)
	if !defined {
		return ""
	}
	return path.Base(frame.File) + ":" + strconv.Itoa(frame.Line)
}

// getCallerFrame gets caller frame. The argument skip is the number of stack
// frames to ascend, with 0 identifying the caller of getCallerFrame. The
// boolean ok is false if it was not possible to recover the information.
//
// Note: This implementation is similar to runtime.Caller, but it returns the whole frame.
func getCallerFrame(skip int) (frame runtime.Frame, ok bool) {
	const skipOffset = 2 // skip getCallerFrame and Callers
	pc := make([]uintptr, 1)
	numFrames := runtime.Callers(skip+skipOffset, pc)
	if numFrames < 1 {
		return
	}
	frame, _ = runtime.CallersFrames(pc).Next()
	return frame, frame.PC != 0
}
