package main

// URILocker locks and unlocks a specific URIs claim on a shared resource represented by a string
type URILocker interface {
	// Lock will store the provided URI in the configured lock store, representing its claim on a shared resource
	Lock(string) (bool, string, error)

	// Unlock will clear the lock so that someone else may obtain it. An error will be returned if the value has changed.
	Unlock(string) (string, error)

	// Read will return the value of the lock or an empty string.
	Read() (string, error)
}

