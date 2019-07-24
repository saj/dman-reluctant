package main

import (
	"io"
	"io/ioutil"
	"log"
	"os"
	"time"

	"github.com/cloudfoundry/jibber_jabber"
	"gopkg.in/alecthomas/kingpin.v2"

	"github.com/saj/dman-reluctant/internal/man"
)

const (
	defaultLanguage = "en"
	defaultCacheTTL = 24 * time.Hour * 7 * 2
)

var cache = &man.Cache{}

func main() {
	log.SetFlags(0)

	var (
		release = kingpin.Flag("release", "").PlaceHolder("SUITE").Default("stable").String()
		page    = kingpin.Arg("page", "").Required().String()
	)

	kingpin.Parse()

	k := man.Key{
		Page: *page,
		Dist: *release,
		Lang: lang(),
	}
	m, err := get(k)
	if err != nil {
		if fetchError, ok := err.(man.FetchError); ok {
			if fetchError.IsNotFound() {
				log.Fatalf("man page not found: %s", k.Page)
			}
		}
		log.Fatal(err)
	}

	err = man.Render(m)
	if err != nil {
		log.Fatal(err)
	}
}

func lang() string {
	l, err := jibber_jabber.DetectLanguage()
	if err != nil {
		return defaultLanguage
	}
	return l
}

func get(k man.Key) (io.ReadCloser, error) {
	c, cerr := cache.Get(k, defaultCacheTTL)
	if cerr == nil {
		return c, nil
	}

	r, rerr := refresh(k)
	if rerr != nil {
		if cerr == man.CacheStaleError {
			log.Printf("falling back to stale cache: %s", rerr)
			return c, nil
		}
		return nil, rerr
	}
	if c != nil {
		c.Close()
	}
	return r, nil
}

func refresh(k man.Key) (io.ReadCloser, error) {
	f, err := fetch(k)
	if err != nil {
		return nil, err
	}

	if err := cache.Put(k, f); err != nil {
		log.Print(err)
	}

	_, err = f.Seek(0, os.SEEK_SET)
	if err != nil {
		f.Close()
		return nil, err
	}
	return f, nil
}

func fetch(k man.Key) (readSeekCloser, error) {
	f, err := mktemp()
	if err != nil {
		return nil, err
	}

	if err := man.Fetch(k, f); err != nil {
		f.Close()
		return nil, err
	}

	_, err = f.Seek(0, os.SEEK_SET)
	if err != nil {
		f.Close()
		return nil, err
	}
	return f, nil
}

type readSeekCloser interface {
	io.Reader
	io.Seeker
	io.Closer
}

func mktemp() (*os.File, error) {
	f, err := ioutil.TempFile("", "")
	if err != nil {
		return nil, err
	}
	if err := os.Remove(f.Name()); err != nil {
		f.Close()
		return nil, err
	}
	return f, nil
}
