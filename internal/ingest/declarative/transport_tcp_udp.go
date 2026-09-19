package declarative

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/Alevsk/respondent/internal/logging"
)

const (
	// defaultMaxMessageBytes is the default maximum message size for TCP/UDP.
	defaultMaxMessageBytes = 65536

	// defaultBufferSize is the default read buffer size.
	defaultBufferSize = 4096

	// defaultDelimiter is the default message delimiter for TCP streams.
	defaultDelimiter = "\n"

	// tcpDialTimeout is the default timeout for TCP dial operations.
	tcpDialTimeout = 30 * time.Second
)

// Compile-time assertion that TCPUDPTransport implements StreamTransport.
var _ StreamTransport = (*TCPUDPTransport)(nil)

func init() {
	RegisterTransport("tcp_udp", newTCPUDPTransport)
}

// TCPUDPTransport implements StreamTransport for raw TCP and UDP socket sources.
// It supports both connect (client) and listen (server) modes.
type TCPUDPTransport struct {
	spec       *TCPUDPSpec
	sourceName string
	logger     *logging.Logger

	delimiter       []byte
	maxMessageBytes int
	bufferSize      int

	mu       sync.Mutex
	conn     net.Conn       // TCP connection (connect or accepted)
	pconn    net.PacketConn // UDP packet connection
	listener net.Listener   // TCP listener (listen mode)
	scanner  *bufio.Scanner // TCP scanner for delimited reads
	done     chan struct{}
	closed   bool
}

// newTCPUDPTransport creates a TCPUDPTransport from a source definition.
func newTCPUDPTransport(def *SourceDefinition, _ map[string]string, _ TokenProvider, _ EnvResolver, logger *logging.Logger) (Transport, error) {
	spec := def.Transport.TCPUDP
	if spec == nil {
		return nil, fmt.Errorf("transport.tcp_udp configuration is required for source %q", def.Name)
	}

	if spec.Protocol == "" {
		return nil, fmt.Errorf("transport.tcp_udp.protocol is required for source %q", def.Name)
	}
	if spec.Mode == "" {
		return nil, fmt.Errorf("transport.tcp_udp.mode is required for source %q", def.Name)
	}
	if spec.Address == "" {
		return nil, fmt.Errorf("transport.tcp_udp.address is required for source %q", def.Name)
	}

	delimiter := []byte(defaultDelimiter)
	if spec.Delimiter != "" {
		delimiter = []byte(spec.Delimiter)
	}

	maxMsg := defaultMaxMessageBytes
	if spec.MaxMessageBytes > 0 {
		maxMsg = spec.MaxMessageBytes
	}

	bufSize := defaultBufferSize
	if spec.BufferSize > 0 {
		bufSize = spec.BufferSize
	}

	return &TCPUDPTransport{
		spec:            spec,
		sourceName:      def.Name,
		logger:          logger,
		delimiter:       delimiter,
		maxMessageBytes: maxMsg,
		bufferSize:      bufSize,
	}, nil
}

// Fetch returns ErrNotPullBased because TCP/UDP is a push-based transport.
func (t *TCPUDPTransport) Fetch(_ context.Context, _, _ string) ([]byte, int, error) {
	return nil, 0, ErrNotPullBased
}

// Connect establishes the TCP or UDP connection based on the configured mode.
func (t *TCPUDPTransport) Connect(ctx context.Context) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.done = make(chan struct{})
	t.closed = false

	t.logger.Info("connecting TCP/UDP transport",
		logging.String("protocol", t.spec.Protocol),
		logging.String("mode", t.spec.Mode),
		logging.String("address", t.spec.Address),
		logging.String("source_name", t.sourceName),
	)

	switch t.spec.Protocol {
	case "tcp":
		return t.connectTCP(ctx)
	case "udp":
		return t.connectUDP()
	default:
		return fmt.Errorf("unsupported protocol %q", t.spec.Protocol)
	}
}

// connectTCP handles TCP connection setup for both connect and listen modes.
func (t *TCPUDPTransport) connectTCP(ctx context.Context) error {
	switch t.spec.Mode {
	case "connect":
		timeout := tcpDialTimeout
		if deadline, ok := ctx.Deadline(); ok {
			timeout = time.Until(deadline)
		}

		conn, err := net.DialTimeout("tcp", t.spec.Address, timeout)
		if err != nil {
			return fmt.Errorf("tcp dial %q: %w", t.spec.Address, err)
		}

		t.conn = conn
		t.scanner = t.newScanner(conn)

		t.logger.Info("TCP connected",
			logging.String("address", t.spec.Address),
			logging.String("source_name", t.sourceName),
		)
		return nil

	case "listen":
		ln, err := net.Listen("tcp", t.spec.Address)
		if err != nil {
			return fmt.Errorf("tcp listen %q: %w", t.spec.Address, err)
		}

		t.listener = ln

		t.logger.Info("TCP listener started",
			logging.String("address", ln.Addr().String()),
			logging.String("source_name", t.sourceName),
		)

		// Accept a single connection in the background.
		go t.acceptLoop(ln)
		return nil

	default:
		return fmt.Errorf("unsupported TCP mode %q", t.spec.Mode)
	}
}

// acceptLoop accepts a single TCP connection from the listener.
func (t *TCPUDPTransport) acceptLoop(ln net.Listener) {
	conn, err := ln.Accept()
	if err != nil {
		select {
		case <-t.done:
			// Listener was closed intentionally.
			return
		default:
		}
		t.logger.Warn("TCP accept failed",
			logging.String("source_name", t.sourceName),
			logging.Err("error", err),
		)
		return
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	// Guard against Close() having been called while we were blocking on Accept.
	select {
	case <-t.done:
		_ = conn.Close()
		return
	default:
	}

	t.conn = conn
	t.scanner = t.newScanner(conn)

	t.logger.Info("TCP client accepted",
		logging.String("remote_addr", conn.RemoteAddr().String()),
		logging.String("source_name", t.sourceName),
	)
}

// connectUDP sets up the UDP packet connection.
func (t *TCPUDPTransport) connectUDP() error {
	switch t.spec.Mode {
	case "listen":
		pc, err := net.ListenPacket("udp", t.spec.Address)
		if err != nil {
			return fmt.Errorf("udp listen %q: %w", t.spec.Address, err)
		}

		t.pconn = pc

		t.logger.Info("UDP listener started",
			logging.String("address", pc.LocalAddr().String()),
			logging.String("source_name", t.sourceName),
		)
		return nil

	case "connect":
		// For UDP "connect", we still use ListenPacket on an ephemeral port.
		// The remote address is resolved and used for filtering if needed.
		pc, err := net.ListenPacket("udp", ":0")
		if err != nil {
			return fmt.Errorf("udp connect listen: %w", err)
		}

		t.pconn = pc

		t.logger.Info("UDP connect mode ready",
			logging.String("remote_address", t.spec.Address),
			logging.String("local_address", pc.LocalAddr().String()),
			logging.String("source_name", t.sourceName),
		)
		return nil

	default:
		return fmt.Errorf("unsupported UDP mode %q", t.spec.Mode)
	}
}

// newScanner creates a bufio.Scanner with the configured delimiter and buffer size.
func (t *TCPUDPTransport) newScanner(conn net.Conn) *bufio.Scanner {
	scanner := bufio.NewScanner(conn)
	buf := make([]byte, t.bufferSize)
	scanner.Buffer(buf, t.maxMessageBytes)

	delim := t.delimiter
	scanner.Split(func(data []byte, atEOF bool) (advance int, token []byte, err error) {
		if atEOF && len(data) == 0 {
			return 0, nil, nil
		}
		if i := bytes.Index(data, delim); i >= 0 {
			return i + len(delim), data[:i], nil
		}
		if atEOF {
			return len(data), data, nil
		}
		// Request more data.
		return 0, nil, nil
	})

	return scanner
}

// Recv blocks until a message is received or the context is cancelled.
func (t *TCPUDPTransport) Recv(ctx context.Context) ([]byte, error) {
	t.mu.Lock()
	protocol := t.spec.Protocol
	t.mu.Unlock()

	switch protocol {
	case "tcp":
		return t.recvTCP(ctx)
	case "udp":
		return t.recvUDP(ctx)
	default:
		return nil, fmt.Errorf("unsupported protocol %q", protocol)
	}
}

// recvTCP reads a delimited message from the TCP connection.
func (t *TCPUDPTransport) recvTCP(ctx context.Context) ([]byte, error) {
	type scanResult struct {
		data []byte
		err  error
	}
	ch := make(chan scanResult, 1)

	go func() {
		t.mu.Lock()
		scanner := t.scanner
		conn := t.conn
		t.mu.Unlock()

		if scanner == nil || conn == nil {
			ch <- scanResult{err: fmt.Errorf("tcp not connected")}
			return
		}

		if scanner.Scan() {
			token := scanner.Bytes()
			data := make([]byte, len(token))
			copy(data, token)
			ch <- scanResult{data: data}
		} else {
			err := scanner.Err()
			if err == nil {
				err = fmt.Errorf("tcp connection closed")
			}
			ch <- scanResult{err: fmt.Errorf("tcp read: %w", err)}
		}
	}()

	select {
	case <-ctx.Done():
		// Close connection to unblock the scanner.
		t.mu.Lock()
		if t.conn != nil {
			_ = t.conn.Close()
		}
		t.mu.Unlock()
		<-ch
		return nil, ctx.Err()
	case res := <-ch:
		return res.data, res.err
	}
}

// recvUDP reads a datagram from the UDP connection.
func (t *TCPUDPTransport) recvUDP(ctx context.Context) ([]byte, error) {
	type readResult struct {
		data []byte
		err  error
	}
	ch := make(chan readResult, 1)

	go func() {
		t.mu.Lock()
		pc := t.pconn
		t.mu.Unlock()

		if pc == nil {
			ch <- readResult{err: fmt.Errorf("udp not connected")}
			return
		}

		buf := make([]byte, t.maxMessageBytes)
		n, _, err := pc.ReadFrom(buf)
		if err != nil {
			ch <- readResult{err: fmt.Errorf("udp read: %w", err)}
			return
		}
		data := make([]byte, n)
		copy(data, buf[:n])
		ch <- readResult{data: data}
	}()

	select {
	case <-ctx.Done():
		t.mu.Lock()
		if t.pconn != nil {
			_ = t.pconn.Close()
		}
		t.mu.Unlock()
		<-ch
		return nil, ctx.Err()
	case res := <-ch:
		return res.data, res.err
	}
}

// Close gracefully shuts down the TCP/UDP transport.
// Safe to call multiple times.
func (t *TCPUDPTransport) Close() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.closed {
		return nil
	}
	t.closed = true

	// Signal done.
	if t.done != nil {
		select {
		case <-t.done:
		default:
			close(t.done)
		}
	}

	var firstErr error

	if t.listener != nil {
		if err := t.listener.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
		t.listener = nil
	}

	if t.conn != nil {
		if err := t.conn.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
		t.conn = nil
	}

	if t.pconn != nil {
		if err := t.pconn.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
		t.pconn = nil
	}

	t.scanner = nil

	t.logger.Info("TCP/UDP transport closed",
		logging.String("source_name", t.sourceName),
	)

	return firstErr
}
