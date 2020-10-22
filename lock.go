package main

import (
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
		resultErr = multierror.Append(resultErr, putErr)
		log.Printf("Error obtaining lock, tryna figure out what the current value is. %+v\n", resultErr.ErrorOrNil())
		value, getErr := ll.dynalock.Get(ll.name)
		if getErr != nil {
			resultErr = multierror.Append(resultErr, getErr)
			log.Printf("Error reading current lock value too. %+v\n", resultErr.ErrorOrNil())
			return false, "", resultErr.ErrorOrNil()
		}
		log.Printf("Current lock value found, still returning an error tho. %+v\n", resultErr.ErrorOrNil())
		return false, string(value.BytesValue()), resultErr.ErrorOrNil()
	}
	log.Printf("Lock should have been obtained: %+v, %+v, %+v", success, uri, resultErr.ErrorOrNil())
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
