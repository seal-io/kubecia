package cache

import (
	"context"
	"errors"
	"fmt"
	"hash/fnv"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/spf13/afero"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/klog/v2"

	"github.com/seal-io/kubecia/pkg/consts"
)

// FileConfig holds the configuration of the filesystem cache,
// entry indexes by key and stores in one file.
type FileConfig struct {
	// Namespace indicates the operating workspace.
	Namespace string
	// EntryMaxAge indicates the maximum lifetime of each entry,
	// default is 15 mins.
	EntryMaxAge time.Duration
	// LazyEntryEviction indicates to evict an expired entry at next peeking,
	// by default, a background looping tries to evict expired entries per 3 mins.
	LazyEntryEviction bool
	// Buckets indicates the bucket number of cache,
	// value must be a power of two,
	// default is 12.
	Buckets int
}

func (c *FileConfig) Default() {
	c.Namespace = strings.TrimSpace(c.Namespace)
	if c.EntryMaxAge == 0 {
		c.EntryMaxAge = 15 * time.Minute
	}

	if c.Buckets == 0 {
		c.Buckets = 12
	}
}

func (c *FileConfig) Validate() error {
	if c.EntryMaxAge < 0 {
		return errors.New("invalid entry max age: negative")
	}

	if c.Buckets < 0 {
		return errors.New("invalid buckets: negative")
	}

	return nil
}

// NewFile returns a filesystem Cache implementation.
func NewFile(ctx context.Context) (Cache, error) {
	return NewFileWithConfig(ctx, FileConfig{})
}

// MustNewFile likes NewFile, but panic if error found.
func MustNewFile(ctx context.Context) Cache {
	n, err := NewFile(ctx)
	if err != nil {
		panic(fmt.Errorf("error creating filesystem cache: %w", err))
	}

	return n
}

const (
	dirPerm  = 0o700
	filePerm = 0o600
	pathSep  = string(filepath.Separator)
)

// NewFileWithConfig returns a filesystem Cache implementation with given configuration.
func NewFileWithConfig(ctx context.Context, cfg FileConfig) (Cache, error) {
	// Default, validate.
	cfg.Default()

	err := cfg.Validate()
	if err != nil {
		return nil, err
	}

	logger := klog.LoggerWithName(klog.Background(), "cache.file")

	// Prepare directories.
	dataDir := consts.DataDir()
	if err = os.MkdirAll(dataDir, dirPerm); err != nil && !os.IsExist(err) {
		return nil, fmt.Errorf("error creating data dir: %w", err)
	}

	for i := 0; i < cfg.Buckets; i++ {
		bucketDir := filepath.Join(dataDir, strconv.FormatInt(int64(i), 10))
		if err = os.MkdirAll(bucketDir, dirPerm); err != nil && !os.IsExist(err) {
			return nil, fmt.Errorf("error creating bucket dir: %w", err)
		}
	}

	// Init.
	underlay := afero.NewBasePathFs(afero.NewOsFs(), dataDir)

	if !cfg.LazyEntryEviction {
		go func() {
			_ = wait.PollUntilContextCancel(ctx, 3*time.Minute, true, func(ctx context.Context) (bool, error) {
				_ = afero.Walk(underlay, pathSep, func(path string, fi os.FileInfo, _ error) error {
					if fi.IsDir() {
						return nil
					}

					if !fi.ModTime().Local().Add(cfg.EntryMaxAge).Before(time.Now()) {
						return nil
					}

					err := underlay.Remove(path)
					if err != nil && !os.IsNotExist(err) {
						logger.Error(err, "error evicting expired entry", "path", path)
					}

					return nil
				})

				return false, nil
			})
		}()
	}

	fc := fileCache{
		logger:     logger,
		underlay:   underlay,
		bucket:     uint64(cfg.Buckets),
		namespace:  cfg.Namespace,
		expiration: cfg.EntryMaxAge,
		lazyEvict:  cfg.LazyEntryEviction,
	}

	return fc, nil
}

// MustNewFileWithConfig likes NewFileWithConfig, but panic if error found.
func MustNewFileWithConfig(ctx context.Context, cfg FileConfig) Cache {
	n, err := NewFileWithConfig(ctx, cfg)
	if err != nil {
		panic(fmt.Errorf("error creating filesystem cache: %w", err))
	}

	return n
}

// fileCache adapts Cache interface to implement a filesystem cache with afero.Fs.
type fileCache struct {
	logger     klog.Logger
	underlay   afero.Fs
	bucket     uint64
	namespace  string
	expiration time.Duration
	lazyEvict  bool
}

func (c fileCache) wrapKey(s *string) *string {
	r := filepath.Join(pathSep, c.namespace, *s)

	h := fnv.New64a()
	_, _ = h.Write([]byte(r))
	p := strconv.FormatUint(h.Sum64()%c.bucket, 10)

	r = filepath.Join(pathSep, p, r)

	return &r
}

func (c fileCache) Close() error {
	return nil
}

func (c fileCache) Name() string {
	return "file"
}

func (c fileCache) Set(ctx context.Context, key string, entry []byte) error {
	wk := c.wrapKey(&key)

	err := c.underlay.MkdirAll(filepath.Dir(*wk), dirPerm)
	if err != nil && !os.IsExist(err) {
		return err
	}

	err = afero.WriteFile(c.underlay, *wk, entry, filePerm)
	if err != nil && !os.IsExist(err) {
		return wrapFileError(err)
	}

	err = c.underlay.Chtimes(*wk, time.Now(), time.Now())
	if err != nil {
		return wrapFileError(err)
	}

	if lg := c.logger.V(5); lg.Enabled() {
		lg.Info("set",
			"key", key, "size", humanize.IBytes(uint64(len(entry))))
	}

	return nil
}

func (c fileCache) Delete(ctx context.Context, key string) ([]byte, error) {
	wk := c.wrapKey(&key)

	entry, err := afero.ReadFile(c.underlay, *wk)
	if err != nil {
		return nil, wrapFileError(err)
	}

	if !c.lazyEvict {
		err = c.underlay.Chtimes(*wk, time.Now(), time.Now().Add(-c.expiration))
	} else {
		err = c.underlay.Remove(*wk)
	}

	if err != nil {
		return nil, wrapFileError(err)
	}

	if lg := c.logger.V(5); err == nil && lg.Enabled() {
		lg.Info("deleted",
			"key", key, "size", humanize.IBytes(uint64(len(entry))))
	}

	return entry, nil
}

func (c fileCache) Get(ctx context.Context, key string) ([]byte, error) {
	wk := c.wrapKey(&key)

	fi, err := c.underlay.Stat(*wk)
	if err != nil {
		if os.IsNotExist(err) {
			c.logger.V(5).Info("missed", "key", key)
		}

		return nil, wrapFileError(err)
	}

	if fi.ModTime().Local().Add(c.expiration).Before(time.Now()) {
		if c.lazyEvict {
			_ = c.underlay.Remove(*wk)
		}

		c.logger.V(5).Info("missed", "key", key)

		return nil, ErrEntryNotFound
	}

	entry, err := afero.ReadFile(c.underlay, *wk)
	if err != nil {
		if os.IsNotExist(err) {
			c.logger.V(5).Info("missed", "key", key)
		}

		return nil, wrapFileError(err)
	}

	if lg := c.logger.V(5); err == nil && lg.Enabled() {
		lg.Info("hit",
			"key", key, "size", humanize.IBytes(uint64(len(entry))))
	}

	return entry, nil
}

func wrapFileError(err error) error {
	switch {
	case err == nil:
		return nil
	case os.IsNotExist(err):
		return ErrEntryNotFound
	case errors.Is(err, io.ErrShortWrite):
		return ErrEntryTooBig
	}

	return err
}
