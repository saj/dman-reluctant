package man

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

const (
	server = "https://dyn.manpages.debian.org"

	requestTimeout  = 10 * time.Second
	bodyLengthLimit = 5 * 1024 * 1024
)

var client = &http.Client{}

type FetchError struct {
	Err   error
	fatal bool
}

func (e FetchError) Error() string {
	return e.Err.Error()
}

func (e FetchError) IsNotFound() bool {
	if httpError, ok := e.Err.(HTTPError); ok {
		return httpError.StatusCode == 404
	}
	return false
}

type HTTPError struct {
	URL        *url.URL
	Status     string
	StatusCode int
}

func (e HTTPError) Error() string {
	return fmt.Sprintf("http: %s: %s", e.URL.String(), e.Status)
}

func Fetch(k Key, w io.Writer) error {
	urls, err := buildCandidateURLs(k)
	if err != nil {
		return err
	}

	for _, u := range urls {
		err = fetchOne(u, w)
		if err != nil {
			if ferr, ok := err.(FetchError); ok {
				if !ferr.fatal {
					continue
				}
			}
			return err
		}
		break
	}
	return err
}

func fetchOne(u *url.URL, w io.Writer) error {
	ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
	defer cancel()

	req := &http.Request{URL: u}
	req = req.WithContext(ctx)
	res, err := client.Do(req)
	if err != nil {
		return FetchError{Err: err}
	}
	defer res.Body.Close()

	makeHTTPError := func() HTTPError {
		return HTTPError{
			URL:        u,
			Status:     res.Status,
			StatusCode: res.StatusCode,
		}
	}

	r := io.LimitReader(res.Body, bodyLengthLimit)

	if res.StatusCode >= 400 && res.StatusCode < 600 {
		io.Copy(&bitbucket{}, r) // maximise chance of transport reuse
		return FetchError{Err: makeHTTPError()}
	}

	n, err := io.Copy(w, r)
	// w is tainted.  All returned errors must be fatal from this point.
	if err != nil {
		return FetchError{Err: err, fatal: true}
	}
	if n == bodyLengthLimit {
		return FetchError{
			Err:   errors.New("fetch: abandoned read: body too large"),
			fatal: true,
		}
	}
	return nil
}

func buildCandidateURLs(k Key) ([]*url.URL, error) {
	var (
		userLang     = k.Lang
		fallbackLang = "en"
	)

	langs := []string{userLang, fallbackLang}
	if userLang == "" || userLang == fallbackLang {
		langs = []string{fallbackLang}
	}

	// Note that the document is not itself in a compressed representation.
	// Browsing to one of these URLs will present you with plaintext roff.
	gz := ".gz"

	s := make([]string, 0, 6)
	for _, lang := range langs {
		s = append(s, server+"/"+k.Dist+"/"+k.Page+"."+lang+gz)
	}
	s = append(s, server+"/"+k.Dist+"/"+k.Page+gz)
	for _, lang := range langs {
		s = append(s, server+"/"+k.Page+"."+lang+gz)
	}
	s = append(s, server+"/"+k.Page+gz)

	u := make([]*url.URL, len(s))
	for i := range s {
		var err error
		u[i], err = url.Parse(s[i])
		if err != nil {
			return nil, err
		}
	}
	return u, nil
}
