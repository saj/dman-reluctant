package man

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

type Cache struct {
	Root string
}

var (
	CacheNotFoundError = errors.New("cache: page not found")
	CacheStaleError    = errors.New("cache: page stale")
)

func (c *Cache) Get(k Key, ttl time.Duration) (io.ReadCloser, error) {
	fmtErr := func(err error) error {
		return fmt.Errorf("cache: %s", err)
	}

	p, err := c.path(k)
	if err != nil {
		return nil, fmtErr(err)
	}

	f, err := os.Open(p)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, CacheNotFoundError
		}
		return nil, fmtErr(err)
	}

	if ttl <= 0 {
		return f, nil
	}

	fi, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, fmtErr(err)
	}
	age := time.Since(fi.ModTime())
	if age > ttl {
		err = CacheStaleError
	}
	return f, err
}

func (c *Cache) Put(k Key, r io.Reader) (err error) {
	fmtErr := func(err error) error {
		return fmt.Errorf("cache: %s", err)
	}

	p, pathError := c.path(k)
	if pathError != nil {
		return fmtErr(pathError)
	}

	if err := os.Remove(p); err != nil { // allow any open fds to shadow
		if !os.IsNotExist(err) {
			return fmtErr(err)
		}
		if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
			return fmtErr(err)
		}
	}

	f, createError := os.Create(p)
	if createError != nil {
		return fmtErr(createError)
	}
	defer func() {
		closeError := f.Close()
		if closeError != nil && err == nil {
			err = fmtErr(closeError)
		}
	}()

	if _, err = io.Copy(f, r); err != nil {
		return fmtErr(err)
	}
	return nil
}

func (c *Cache) path(k Key) (string, error) {
	root := c.Root
	if root == "" {
		var err error
		root, err = c.defaultRoot()
		if err != nil {
			return "", err
		}
	}
	return filepath.Join(root, k.Dist, k.Lang, k.Page), nil
}

func (c *Cache) defaultRoot() (string, error) {
	h, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("root directory unknown: %s", err)
	}
	return filepath.Join(h, ".dman/cache"), nil
}
