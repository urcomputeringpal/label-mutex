package main

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"os"

	"github.com/motemen/go-loghttp"
	"github.com/urcomputeringpal/label-mutex/gcslock"
)

type gcsLocker struct {
	lock gcslock.ContextLocker
	name string
}

type customTransport struct {
	Transport http.RoundTripper
}

func (c *customTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.URL.Host == "storage.googleapis.com" {
		newURL, _ := url.Parse("https://fake-gcs-server:4443")
		req.URL.Scheme = newURL.Scheme
		req.URL.Host = newURL.Host
	}
	return c.Transport.RoundTrip(req)
}

func NewGCSLocker(bucket string, name string) (*gcsLocker, err error) {
	var locker gcslock.ContextLocker

	customEndpoint := os.Getenv("GCS_ENDPOINT_URL")
	if customEndpoint != "" {

		locker = gcslock.NewWithClient(&http.Client{Transport: &customTransport{Transport: &loghttp.Transport{}}}, bucket, name)
	} else {
		locker, err = gcslock.New(context.Background(), bucket, name)
		if err != nil {
			return nil, err
		}
	}
	ll := &gcsLocker{
		lock: locker,
		name: name,
	}
	return ll, nil
}

func (ll *gcsLocker) Lock(uri string) (bool, string, error) {
	return false, "", errors.New("unimplemented")

}

func (ll *gcsLocker) Unlock(uri string) (string, error) {
	return "", errors.New("unimplemented")
}

func (ll *gcsLocker) Read() (string, error) {
	return "", errors.New("unimplemented")
}
