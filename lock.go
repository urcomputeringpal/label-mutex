package main

import (
	"fmt"
	"log"
	"os"

	"github.com/aws/aws-sdk-go/aws"
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
	Unlock(string) (string, error)

	// Read will return the value of the lock or an empty string.
	Read() (string, error)
}

// NewDynamoURILocker initializes a URILocker
func NewDynamoURILocker(table string, partition string, name string) (URILocker, error) {
	sess, err := session.NewSession()
	if err != nil {
		return nil, fmt.Errorf("failed to create AWS session: %+v", err)
	}

	var d dynalock.Store

	customEndpoint := os.Getenv("AWS_DYNAMODB_ENDPOINT_URL")
	if customEndpoint != "" {
		d = dynalock.New(dynamodb.New(sess, &aws.Config{
			Endpoint: aws.String(customEndpoint),
			Region:   aws.String(os.Getenv("AWS_DEFAULT_REGION")),
		}), table, partition)
	} else {
		d = dynalock.New(dynamodb.New(sess, &aws.Config{
			Region: aws.String(os.Getenv("AWS_DEFAULT_REGION")),
		}), table, partition)
	}

	ll := &uriLocker{
		dynalock: d,
		name:     name,
	}

	return ll, nil
}

func (ll *uriLocker) Lock(uri string) (bool, string, error) {
	log.Printf("Attempting to lock %s with value of %s ...\n", ll.name, uri)
	var resultErr *multierror.Error
	success, value, firstPutErr := ll.dynalock.AtomicPut(ll.name, dynalock.WriteWithNoExpires(), dynalock.WriteWithBytes([]byte(uri)))
	if firstPutErr != nil {
		resultErr = multierror.Append(resultErr, firstPutErr)
		log.Printf("Couldn't obtain lock outright, trying figure out what the current value is. %+v\n", resultErr.ErrorOrNil())
		value, getErr := ll.dynalock.Get(ll.name)
		if getErr != nil {
			resultErr = multierror.Append(resultErr, getErr)
			log.Printf("Error reading current lock value too. %+v\n", resultErr.ErrorOrNil())
			return false, "", resultErr.ErrorOrNil()
		}
		if string(value.BytesValue()) == uri {
			success, value, putErr := ll.dynalock.AtomicPut(ll.name, dynalock.WriteWithNoExpires(), dynalock.WriteWithBytes([]byte(uri)), dynalock.WriteWithPreviousKV(value))
			if putErr == nil {
				log.Printf("Lock confirmed: %+v, %+v, %+v", success, value, resultErr.ErrorOrNil())
				return false, uri, nil
			}
			resultErr = multierror.Append(resultErr, putErr)
			log.Printf("Error confirming lock: %+v, %+v, %+v", success, value, resultErr.ErrorOrNil())
			return false, "", resultErr.ErrorOrNil()
		}
		log.Printf("Lock value mismatch found. %+v\n", resultErr.ErrorOrNil())
		return false, string(value.BytesValue()), nil
	}
	log.Printf("Lock obtained: %+v, %+v, %+v", success, value, resultErr.ErrorOrNil())
	return success, uri, resultErr.ErrorOrNil()
}

func (ll *uriLocker) Unlock(uri string) (string, error) {
	log.Printf("Attempting to unlock %s with value of %s ...\n", ll.name, uri)
	value, getErr := ll.dynalock.Get(ll.name)
	if getErr != nil {
		return "", getErr
	}
	currentLockHolder := string(value.BytesValue())
	if currentLockHolder != uri {
		return currentLockHolder, fmt.Errorf("Couldn't unlock with provided value of %s, lock currently held by %s", uri, currentLockHolder)
	}
	_, err := ll.dynalock.AtomicDelete(ll.name, value)
	return "", err
}

func (ll *uriLocker) Read() (string, error) {
	value, getErr := ll.dynalock.Get(ll.name)
	if getErr != nil {
		return "", getErr
	}
	return string(value.BytesValue()), nil
}
