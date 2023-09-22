package main

import (
	"context"
	"errors"

	"github.com/urcomputeringpal/label-mutex/gcslock"
)

type gcsLocker struct {
	lock gcslock.ContextLocker
	name string
}

func NewGCSLocker(bucket string, name string) (*gcsLocker, error) {
	locker, err := gcslock.New(context.Background(), bucket, name)
	if err != nil {
		return nil, err
	} else {
		ll := &gcsLocker{
			lock: locker,
			name: name,
		}
		return ll, nil
	}
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
