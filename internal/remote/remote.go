// Package remote lets a running mado instance be driven from another
// process: `mado --remote open FILE` hands the file to the instance
// already on screen instead of starting a second one.
//
// Every instance listens on a Unix domain socket named after its pid,
// in a per-user directory. Clients pick the most recently started
// instance that still answers, or the one named by MADO_SOCKET.
package remote

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Commands an instance understands.
const (
	// CmdOpen opens the file in a tab, or activates the tab that
	// already holds it.
	CmdOpen = "open"
	// CmdFocus switches to the file's tab, and fails if the file is
	// not open. Unlike CmdOpen it never adds a tab.
	CmdFocus = "focus"
)

// EnvSocket names a specific socket, for both ends: an instance
// listens there instead of picking its own path, and clients talk to
// it instead of searching. Useful when several instances are running.
const EnvSocket = "MADO_SOCKET"

// Timeout bounds every step of a remote exchange, so neither end hangs
// on an instance that has stopped drawing.
const Timeout = 5 * time.Second

// ErrNoInstance reports that no running instance answered.
var ErrNoInstance = errors.New("no running mado instance")

// Request is one command received from another process. Handlers must
// call Respond exactly once; the sender is waiting for it.
type Request struct {
	Cmd  string `json:"cmd"`
	Path string `json:"path"`

	reply chan string
}

type response struct {
	OK    bool   `json:"ok"`
	Error string `json:"error,omitempty"`
}

// Respond reports the outcome back to the process that sent the
// request. A nil error means it was carried out.
func (r *Request) Respond(err error) {
	msg := ""
	if err != nil {
		msg = err.Error()
	}
	select {
	case r.reply <- msg:
	default:
		// Already answered, or the sender gave up waiting.
	}
}

// Server accepts commands for the running instance.
type Server struct {
	ln       net.Listener
	path     string
	requests chan *Request
	done     chan struct{}
}

// Listen starts a server on path, creating its directory. An existing
// socket file at path is replaced: it belongs to an instance that did
// not clean up after itself.
func Listen(path string) (*Server, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, err
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	ln, err := net.Listen("unix", path)
	if err != nil {
		return nil, err
	}
	// Belt and braces: the directory is already private, but the
	// socket should not be reachable by other users either.
	_ = os.Chmod(path, 0o600)

	s := &Server{
		ln:       ln,
		path:     path,
		requests: make(chan *Request),
		done:     make(chan struct{}),
	}
	go s.accept()
	return s, nil
}

// Path is the socket the server is listening on.
func (s *Server) Path() string { return s.path }

// Next blocks for the next request. It reports false once the server
// is closed.
func (s *Server) Next() (*Request, bool) {
	select {
	case req := <-s.requests:
		return req, true
	case <-s.done:
		return nil, false
	}
}

// Close stops the server and removes its socket file.
func (s *Server) Close() error {
	select {
	case <-s.done:
		return nil
	default:
		close(s.done)
	}
	err := s.ln.Close()
	if rmErr := os.Remove(s.path); rmErr != nil && !os.IsNotExist(rmErr) && err == nil {
		err = rmErr
	}
	return err
}

func (s *Server) accept() {
	for {
		conn, err := s.ln.Accept()
		if err != nil {
			return
		}
		go s.serve(conn)
	}
}

func (s *Server) serve(conn net.Conn) {
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(Timeout))

	var req Request
	if err := json.NewDecoder(conn).Decode(&req); err != nil {
		writeResponse(conn, fmt.Errorf("bad request: %w", err))
		return
	}
	req.reply = make(chan string, 1)

	select {
	case s.requests <- &req:
	case <-s.done:
		writeResponse(conn, errors.New("mado is shutting down"))
		return
	case <-time.After(Timeout):
		writeResponse(conn, errors.New("mado is not responding"))
		return
	}

	select {
	case msg := <-req.reply:
		if msg == "" {
			writeResponse(conn, nil)
		} else {
			writeResponse(conn, errors.New(msg))
		}
	case <-s.done:
		writeResponse(conn, errors.New("mado is shutting down"))
	case <-time.After(Timeout):
		writeResponse(conn, errors.New("mado is not responding"))
	}
}

func writeResponse(conn net.Conn, err error) {
	resp := response{OK: err == nil}
	if err != nil {
		resp.Error = err.Error()
	}
	_ = json.NewEncoder(conn).Encode(resp)
}

// ── client ──────────────────────────────────────────────────────────

// Send delivers cmd for path to a running instance and returns what it
// answered. It returns ErrNoInstance when nothing is listening, which
// is the caller's cue to start mado normally instead.
func Send(dir, cmd, path string) error {
	for _, sock := range candidates(dir) {
		conn, err := net.DialTimeout("unix", sock, Timeout)
		if err != nil {
			// Nothing is listening: the instance died without
			// cleaning up. Tidy up so the next call is faster.
			if os.Getenv(EnvSocket) == "" {
				_ = os.Remove(sock)
			}
			continue
		}
		return exchange(conn, Request{Cmd: cmd, Path: path})
	}
	return ErrNoInstance
}

func exchange(conn net.Conn, req Request) error {
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(Timeout))

	if err := json.NewEncoder(conn).Encode(req); err != nil {
		return err
	}
	var resp response
	if err := json.NewDecoder(conn).Decode(&resp); err != nil {
		return err
	}
	if !resp.OK {
		if resp.Error == "" {
			return errors.New("mado rejected the request")
		}
		return errors.New(resp.Error)
	}
	return nil
}

// candidates lists the sockets to try, most recently started first.
func candidates(dir string) []string {
	if sock := os.Getenv(EnvSocket); sock != "" {
		return []string{sock}
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	type sock struct {
		path    string
		started time.Time
	}
	var socks []sock
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".sock") {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		socks = append(socks, sock{filepath.Join(dir, e.Name()), info.ModTime()})
	}
	sort.Slice(socks, func(i, j int) bool { return socks[i].started.After(socks[j].started) })

	paths := make([]string, len(socks))
	for i, s := range socks {
		paths[i] = s.path
	}
	return paths
}

// ── paths ───────────────────────────────────────────────────────────

// DefaultDir is where instances keep their sockets: $XDG_RUNTIME_DIR
// /mado, or a per-user directory under the system temp dir when
// XDG_RUNTIME_DIR is unset.
func DefaultDir() string {
	if base := os.Getenv("XDG_RUNTIME_DIR"); base != "" {
		return filepath.Join(base, "mado")
	}
	// The temp dir is shared between users on Unix, so the uid keeps
	// instances apart. Windows reports -1 and has a per-user temp dir
	// already.
	if uid := os.Getuid(); uid >= 0 {
		return filepath.Join(os.TempDir(), fmt.Sprintf("mado-%d", uid))
	}
	return filepath.Join(os.TempDir(), "mado")
}

// DefaultPath is the socket this process should listen on: the one
// named by MADO_SOCKET, or one named after the pid in DefaultDir.
func DefaultPath() string {
	if sock := os.Getenv(EnvSocket); sock != "" {
		return sock
	}
	return filepath.Join(DefaultDir(), fmt.Sprintf("%d.sock", os.Getpid()))
}
