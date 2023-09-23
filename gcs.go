package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/hashicorp/go-multierror"
	"github.com/motemen/go-loghttp"
	"github.com/urcomputeringpal/label-mutex/gcslock"
)

type gcsLocker struct {
	lock   gcslock.ContextLocker
	client *http.Client
	name   string
	bucket string
}

type customTransport struct {
	Transport http.RoundTripper
	Endpoint  string
}

func (c *customTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.URL.Host == "storage.googleapis.com" {
		newURL, _ := url.Parse(c.Endpoint)
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
		// create a transport that skips TLS verification
		insecure := &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: os.Getenv("GCS_INSECURE_SKIP_VERIFY") == "true"},
		}
		client = &http.Client{
			Transport: &customTransport{
				Endpoint: customEndpoint,
				Transport: &loghttp.Transport{
					Transport: insecure,
				},
			},
		}
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
		bucket: bucket,
	}
	return ll, nil
}

func (ll *gcsLocker) Lock(uri string) (bool, string, error) {
	contextWithTimeout, cancel := context.WithTimeout(context.Background(), time.Second*30)
	defer cancel()
	log.Printf("Attempting to lock %s with value of %s ...\n", ll.name, uri)
	var resultErr *multierror.Error
	fistWriteErr := ll.lock.ContextLockWithValue(contextWithTimeout, uri)
	if fistWriteErr != nil {
		resultErr = multierror.Append(resultErr, fistWriteErr)
		log.Printf("couldn't obtain lock outright, trying figure out what the current value is. %+v\n", resultErr.ErrorOrNil())
		value, getErr := ll.Read()
		if getErr != nil {
			resultErr = multierror.Append(resultErr, getErr)
			log.Printf("Error reading current lock value too. %+v\n", resultErr.ErrorOrNil())
			return false, "", resultErr.ErrorOrNil()
		}
		if value == uri {
			confirmErr := ll.lock.ContextLockWithValue(contextWithTimeout, uri)
			if confirmErr == nil {
				log.Printf("Lock confirmed: %+v, %+v", value, resultErr.ErrorOrNil())
				return false, uri, nil
			}
			resultErr = multierror.Append(resultErr, confirmErr)
			log.Printf("Error confirming lock: %+v, %+v", value, resultErr.ErrorOrNil())
			return false, "", resultErr.ErrorOrNil()
		}
		log.Printf("Lock value mismatch found. %+v\n", resultErr.ErrorOrNil())
		return false, value, nil
	}
	log.Printf("Lock obtained: %+v, %+v", uri, resultErr.ErrorOrNil())
	return true, uri, resultErr.ErrorOrNil()
}

func (ll *gcsLocker) Unlock(uri string) (string, error) {
	log.Printf("Attempting to unlock %s with value of %s ...\n", ll.name, uri)
	value, getErr := ll.Read()
	if getErr != nil {
		return "", getErr
	}
	if value != uri {
		return value, fmt.Errorf("couldn't unlock with provided value of %s, lock currently held by %s", uri, value)
	}
	log.Printf("Lock confirmed, unlocking...")
	contextWithTimeout, cancel := context.WithTimeout(context.Background(), time.Second*30)
	defer cancel()
	err := ll.lock.ContextUnlock(contextWithTimeout)
	if err != nil {
		return "", err
	} else {
		return "", nil
	}
}

func (ll *gcsLocker) Read() (string, error) {
	contextWithTimeout, cancel := context.WithTimeout(context.Background(), time.Second*30)
	defer cancel()
	return ll.lock.ReadValue(contextWithTimeout, ll.bucket, ll.name)
}
