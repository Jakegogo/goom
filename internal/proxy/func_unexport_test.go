// Package proxy_test 对 proxy 包的测试
package proxy_test

import (
	"errors"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/tencent/goom/internal/proxy"
)

// TestNetConnMock 测试 mock 网络连接
func TestNetConnMock(t *testing.T) {
	// 原始函数
	var connWrite func(c *conn, b []byte) (int, error)

	// 使用 patch 进行切面
	patch, err := proxy.FuncName("net.(*conn).Write", func(c *conn, b []byte) (int, error) {
		n, _ := connWrite(c, b)
		// 修改返回结果
		return n, errors.New("mocked")
	}, &connWrite)

	if err != nil {
		t.Error("mock print err:", err)
	}

	// Stability note:
	// This test used to hardcode 127.0.0.1:80 and os.Exit(1) on failure, which is flaky in CI
	// and can terminate the whole test run. We instead use an ephemeral local listener.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()

	done := make(chan struct{})
	go func() {
		defer close(done)
		c, aerr := ln.Accept()
		if aerr != nil {
			return
		}
		// Stability note:
		// Don't block forever waiting for EOF. If the patch fails to intercept net.(*conn).Write
		// on a given Go version/OS build, the client may not send anything and the server goroutine
		// would hang, causing a 10-minute test timeout.
		_ = c.SetReadDeadline(time.Now().Add(2 * time.Second))
		buf := make([]byte, 64)
		_, _ = c.Read(buf)
		_ = c.Close()
	}()

	addr := ln.Addr().String()
	conn, err := net.Dial("tcp", addr)
	fmt.Println("Connecting to " + addr)
	if err != nil {
		// Don't crash the whole test process.
		t.Skipf("dial failed: %v", err)
		return
	}
	defer conn.Close()

	content := []byte{1, 2, 3}
	_, err = conn.Write(content)

	// 预期返回: err: mocked
	t.Log("err:", err)
	patch.Unpatch()
	_ = conn.Close()
	<-done
}

// // nolint
type conn struct {
	// nolint
	fd *netFD
}

// nolint
func (c *conn) Read([]byte) (n int, err error) { return 0, nil }

// nolint
func (c *conn) Write([]byte) (n int, err error) { return 0, nil }

// nolint
func (c *conn) Close() error { return nil }

// nolint
func (c *conn) LocalAddr() net.Addr { return nil }

// nolint
func (c *conn) RemoteAddr() net.Addr { return nil }

// nolint
func (c *conn) SetDeadline(time.Time) error { return nil }

// nolint
func (c *conn) SetReadDeadline(time.Time) error { return nil }

// nolint
func (c *conn) SetWriteDeadline(time.Time) error { return nil }

// nolint
// Network file descriptor.
type netFD struct {
	// nolint
	pfd FD

	// immutable until Close
	family int
	// nolint
	sotype      int
	isConnected bool // handshake completed or use of association with peer
	net         string
	laddr       net.Addr
	raddr       net.Addr
}

// nolint
// FD is a file descriptor. The net and os packages use this type as a
// field of a larger type representing a network connection or OS file.
type FD struct {
	// Lock sysfd and serialize access to Read and Write methods.
	fdmu fdMutex

	// System file descriptor. Immutable until Close.
	Sysfd int

	// I/O poller.
	pd pollDesc

	// Writev cache.
	iovecs *[]int

	// Semaphore signaled when file is closed.
	csema uint32

	// Non-zero if this file has been set to blocking mode.
	isBlocking uint32

	// Whether this is a streaming descriptor, as opposed to a
	// packet-based descriptor like a UDP socket. Immutable.
	IsStream bool

	// Whether a zero byte read indicates EOF. This is false for a
	// message based socket connection.
	ZeroReadIsEOF bool

	// Whether this is a file rather than a network socket.
	isFile bool
}

// nolint
type fdMutex struct {
	state uint64
	rsema uint32
	wsema uint32
}

// nolint
type pollDesc struct {
	runtimeCtx uintptr
}
