package main

import (
	"errors"
	"fmt"
	"log"

	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/hashicorp/go-multierror"
	"github.com/wolfeidau/dynalock"
)

type uriLocker struct {
	dynalock dynalock.Store
	name     string
}

// URILocker locks and unlocks a specific URIs claim on a shared resource represented by a string
type URILocker interface {
	// Lock will store the provided URI in the configured lock store, representing its claim on a shared resource
	Lock(string) (bool, string, error)

	// Unlock will clear the lock so that someone else may obtain it. An error will be returned if the value has changed.
	Unlock(string) error
}

// NewDynamoURILocker initializes a URILocker
func NewDynamoURILocker(table string, column string, name string) (URILocker, error) {
	sess, err := session.NewSession()
	if err != nil {
		return nil, fmt.Errorf("failed to create AWS session: %+v", err)
	}

	d := dynalock.New(dynamodb.New(sess), table, column)

	ll := &uriLocker{
		dynalock: d,
		name:     name,
	}

	return ll, nil
}

func (ll *uriLocker) Lock(uri string) (bool, string, error) {
	log.Printf("Attempting to lock %s with value of %s ...\n", ll.name, uri)
	var resultErr *multierror.Error
	success, _, putErr := ll.dynalock.AtomicPut(ll.name, dynalock.WriteWithBytes([]byte(uri)))
	if putErr != nil {
		multierror.Append(resultErr, putErr)
		value, getErr := ll.dynalock.Get(ll.name)
		if getErr != nil {
			multierror.Append(resultErr, getErr)
			return false, "", resultErr.ErrorOrNil()
		}
		return false, string(value.BytesValue()), resultErr.ErrorOrNil()
	}
	// TODO this isn't being hit
	if !success && uri == "" && resultErr.ErrorOrNil() == nil {
		return false, "", errors.New("Unknown error setting lock. Please confirm AWS environment variables are configured appropriately")
	}
	return success, uri, resultErr.ErrorOrNil()
}

func (ll *uriLocker) Unlock(uri string) error {
	log.Printf("Attempting to unlock %s with value of %s ...\n", ll.name, uri)
	value, getErr := ll.dynalock.Get(ll.name)
	if getErr != nil {
		return getErr
	}
	currentLockHolder := string(value.BytesValue())
	if currentLockHolder != uri {
		return fmt.Errorf("Couldn't unlock with provided value of %s, lock currently held by %s", uri, currentLockHolder)
	}
	_, err := ll.dynalock.AtomicDelete(ll.name, value)
	return err
}
