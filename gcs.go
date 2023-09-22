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
	lock   gcslock.ContextLocker
	client *http.Client
	name   string
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

func NewGCSLocker(bucket string, name string) (ll *gcsLocker, err error) {
	var locker gcslock.ContextLocker
	var client *http.Client

	customEndpoint := os.Getenv("GCS_ENDPOINT_URL")
	if customEndpoint != "" {
		client = &http.Client{Transport: &customTransport{Transport: &loghttp.Transport{}}}
		locker = gcslock.NewWithClient(client, bucket, name)
	} else {
		client = http.DefaultClient
		locker, err = gcslock.New(context.Background(), bucket, name)
		if err != nil {
			return nil, err
		}
	}
	ll = &gcsLocker{
		lock:   locker,
		client: client,
		name:   name,
	}
	return ll, nil
}

func (ll *gcsLocker) Lock(uri string) (bool, string, error) {
	return false, "", errors.New("unimplemented")

}

func (ll *gcsLocker) Unlock(uri string) (string, error) {
	ll.lock.Unlock()
	return "", errors.New("unimplemented")
}

func (ll *gcsLocker) Read() (string, error) {
	return "", errors.New("unimplemented")
}
