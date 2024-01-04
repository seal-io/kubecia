package server

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/pflag"
	"k8s.io/klog/v2"

	"github.com/seal-io/kubecia/pkg/cache"
	"github.com/seal-io/kubecia/pkg/consts"
)

type (
	ServeOptions struct {
		Cache cache.Cache
	}

	ServeFunc  = func(context.Context, *http.ServeMux, ServeOptions) error
	ServeFuncs = []ServeFunc

	Server struct {
		Socket     string
		ServeFuncs ServeFuncs
	}
)

func (s *Server) AddFlags(flags *pflag.FlagSet) {
	flags.StringVar(&s.Socket, "socket", consts.SocketPath(), "Socket path")
}

func (s *Server) Serve(ctx context.Context) error {
	ls, err := newUnixListener(s.Socket)
	if err != nil {
		return fmt.Errorf("error creating unix listener %s: %w", s.Socket, err)
	}

	defer func() {
		_ = ls.Close()
	}()

	c, err := cache.NewMemory(ctx)
	if err != nil {
		return fmt.Errorf("error creating cache: %w", err)
	}

	defer func() {
		_ = c.Close()
	}()

	m := http.NewServeMux()
	o := ServeOptions{
		Cache: cache.WithSingleFlight(c),
	}

	for i := range s.ServeFuncs {
		err = s.ServeFuncs[i](ctx, m, o)
		if err != nil {
			return fmt.Errorf("error serving: %w", err)
		}
	}

	logger := klog.LoggerWithName(klog.Background(), "http")

	srv := http.Server{
		Handler:      m,
		ReadTimeout:  1 * time.Second,
		WriteTimeout: 5 * time.Second,
		ErrorLog:     log.New(httpLogger(logger), "", 0),
	}

	go func() {
		<-ctx.Done()

		_ = srv.Close()
	}()

	err = srv.Serve(ls)
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}

	return nil
}

func (s *Server) Register(f ServeFunc) {
	if f == nil {
		return
	}

	s.ServeFuncs = append(s.ServeFuncs, f)
}

func newUnixListener(sock string) (net.Listener, error) {
	err := os.MkdirAll(filepath.Dir(sock), 0o700)
	if err != nil && !os.IsExist(err) {
		return nil, fmt.Errorf("error creating unix socket dir: %w", err)
	}

	err = syscall.Unlink(sock)
	if err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("error unlinking unix socket: %w", err)
	}

	ls, err := net.ListenUnix("unix", &net.UnixAddr{Net: "unix", Name: sock})
	if err != nil {
		return nil, fmt.Errorf("error creating unix socket listener: %w", err)
	}

	err = os.Chmod(sock, 0o777)
	if err != nil {
		_ = ls.Close()
		return nil, fmt.Errorf("error chmoding unix socket: %w", err)
	}

	return ls, nil
}

type httpLogger klog.Logger

func (l httpLogger) Write(p []byte) (int, error) {
	// Trim the trailing newline.
	s := strings.TrimSuffix(string(p), "\n")

	if !strings.Contains(s, "broken pipe") {
		klog.Logger(l).Info(s)
	}

	return len(p), nil
}
