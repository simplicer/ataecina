// Package codexappserver asks the official `codex app-server` for the
// account's rate limits over stdio JSON-RPC (SPECS §3.2). Codex makes the
// network call with its own login; Ataecina never sees a token. Only the
// methods on the allowlist can ever be sent.
package codexappserver

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"syscall"
	"time"

	"codeberg.org/simplicer/ataecina/internal/platform/officialcli"
)

const (
	methodInitialize  = "initialize"
	methodInitialized = "initialized"
	methodReadLimits  = "account/rateLimits/read"

	// maxLineBytes caps one message from the server; maxReadBytes caps
	// everything read in one session.
	maxLineBytes = 1 << 20
	maxReadBytes = 4 << 20
)

// allowedMethods is the complete list of what Ataecina may send: the
// initialize handshake (request and its acknowledgement notification) and
// the read of the rate limits. Never account/rateLimitResetCredit/consume,
// login, logout, config writes or anything else that changes state.
var allowedMethods = map[string]bool{
	methodInitialize:  true,
	methodInitialized: true,
	methodReadLimits:  true,
}

var (
	errForbiddenMethod = errors.New("method not on the allowlist")
	errNoAnswer        = errors.New("codex app-server closed without answering")
)

// rpcError is a JSON-RPC error answer.
type rpcError struct {
	Code int `json:"code"`
}

func (e rpcError) Error() string { return fmt.Sprintf("codex app-server answered error %d", e.Code) }

type message struct {
	Method string          `json:"method,omitempty"`
	ID     *int            `json:"id,omitempty"`
	Params any             `json:"params,omitempty"`
	Result json.RawMessage `json:"result,omitempty"`
	Error  *rpcError       `json:"error,omitempty"`
}

// session is one short-lived `codex app-server` process.
type session struct {
	stdin  io.WriteCloser
	stdout *bufio.Reader
	read   int
}

// readRateLimits starts `codex app-server` with a fixed argv and no shell,
// once no other official CLI runs, performs the handshake, reads the
// limits and stops the process.
func readRateLimits(ctx context.Context, codexPath, version string) (json.RawMessage, error) {
	release, err := officialcli.Turn(ctx)
	if err != nil {
		return nil, err
	}
	defer release()
	ctx, stop := context.WithCancel(ctx)
	defer stop()
	cmd := exec.CommandContext(ctx, codexPath, "app-server")
	cmd.Stderr = io.Discard
	cmd.Cancel = func() error { return cmd.Process.Signal(syscall.SIGTERM) }
	cmd.WaitDelay = 2 * time.Second
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	defer func() {
		// Close stdin, ask the process to stop (SIGTERM, SIGKILL after
		// WaitDelay) and reap it.
		_ = stdin.Close()
		stop()
		_ = cmd.Wait()
	}()
	s := &session{stdin: stdin, stdout: bufio.NewReaderSize(stdout, maxLineBytes)}

	clientInfo := map[string]any{"clientInfo": map[string]string{"name": "ataecina", "title": "Ataecina", "version": version}}
	if _, err := s.call(1, methodInitialize, clientInfo); err != nil {
		return nil, err
	}
	if err := s.send(message{Method: methodInitialized}); err != nil {
		return nil, err
	}
	return s.call(2, methodReadLimits, nil)
}

// call sends a request and waits for the answer with the same id,
// ignoring notifications and server requests in between.
func (s *session) call(id int, method string, params any) (json.RawMessage, error) {
	if err := s.send(message{Method: method, ID: &id, Params: params}); err != nil {
		return nil, err
	}
	for {
		line, err := s.nextLine()
		if err != nil {
			return nil, err
		}
		var answer message
		if json.Unmarshal(line, &answer) != nil || answer.Method != "" || answer.ID == nil || *answer.ID != id {
			continue
		}
		if answer.Error != nil {
			return nil, *answer.Error
		}
		return answer.Result, nil
	}
}

func (s *session) send(m message) error {
	if !allowedMethods[m.Method] {
		return fmt.Errorf("%w: %q", errForbiddenMethod, m.Method)
	}
	data, err := json.Marshal(m)
	if err != nil {
		return err
	}
	_, err = s.stdin.Write(append(data, '\n'))
	return err
}

// nextLine returns the next complete line, skipping lines longer than
// maxLineBytes, until maxReadBytes have been read.
func (s *session) nextLine() ([]byte, error) {
	tooLong := false
	for s.read < maxReadBytes {
		chunk, err := s.stdout.ReadSlice('\n')
		s.read += len(chunk)
		switch {
		case errors.Is(err, bufio.ErrBufferFull):
			tooLong = true
		case err != nil:
			return nil, errNoAnswer
		case tooLong:
			tooLong = false
		default:
			return bytes.TrimSpace(chunk), nil
		}
	}
	return nil, errNoAnswer
}
