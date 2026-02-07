package erro

import (
	"fmt"
	"runtime"
)

// TraceableError 可跟踪的错误，异常转述包装
type TraceableError struct {
	// 错误信息
	err    error
	errStr string
	stacks []Frame
	// cause 错误原因
	cause error
}

// Error 错误描述
func (w *TraceableError) Error() string {
	s := ""
	if w.err != nil {
		s += w.err.Error()
	}
	if w.errStr != "" {
		s += w.errStr
	}
	if w.stacks != nil {
		for _, f := range w.stacks {
			s = s + "\n\t" + f.String()
		}
	}
	if w.cause != nil {
		s = s + "\n\ncause by: " + w.cause.Error()
	}
	return s
}

// Cause 获取错误的原因
func (w *TraceableError) Cause() error {
	return w.cause
}

func (w *TraceableError) StackTrace() []Frame {
	return w.stacks
}

// Frame is a single step in stack trace.
type Frame struct {
	// Func contains a function name.
	Func string
	// Line contains a line number.
	Line int
	// Path contains a file path.
	Path string
}

// String formats Frame to string.
func (f Frame) String() string {
	return fmt.Sprintf("%s:%d\n%s()", f.Path, f.Line, f.Func)
}

// NewTraceableErrors 通过 string 描述创建可跟踪的错误
func NewTraceableErrors(errStr string) error {
	return &TraceableError{
		errStr: errStr,
		stacks: trace(2),
	}
}

// NewTraceableErrorc 通过 string, cause 描述创建可跟踪的错误
func NewTraceableErrorc(errStr string, cause error) error {
	return &TraceableError{
		errStr: errStr,
		stacks: trace(2),
		cause:  cause,
	}
}

// NewTraceableError 通过 error, cause 描述创建可跟踪的错误
func NewTraceableError(err, cause error) error {
	return &TraceableError{
		err:    err,
		stacks: trace(2),
		cause:  cause,
	}
}

func trace(skip int) []Frame {
	frames := make([]Frame, 0, 10)
	for {
		pc, path, line, ok := runtime.Caller(skip)
		if !ok {
			break
		}
		fn := runtime.FuncForPC(pc)
		frame := Frame{
			Func: fn.Name(),
			Line: line,
			Path: path,
		}
		frames = append(frames, frame)
		skip++
	}
	return frames
}
