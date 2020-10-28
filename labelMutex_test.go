package main

import (
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"testing"

	"github.com/google/uuid"

	"github.com/google/go-github/v32/github"
)

var (
	http200 = &github.Response{
		Response: &http.Response{StatusCode: 200},
	}
	http404 = &github.Response{
		Response: &http.Response{StatusCode: 404},
	}
)

type happyPathLabelClient struct{}

func (c *happyPathLabelClient) AddLabelsToIssue(ctx context.Context, owner string, repo string, number int, labels []string) ([]*github.Label, *github.Response, error) {
	return nil, http200, nil
}
func (c *happyPathLabelClient) RemoveLabelForIssue(ctx context.Context, owner string, repo string, number int, label string) (*github.Response, error) {
	return http200, nil
}

type racyMockLocker struct {
	value string
}

func (l *racyMockLocker) Lock(v string) (bool, string, error) {
	if l.value == "" {
		l.value = v
		return true, v, nil
	}
	return false, l.value, errors.New("lock already held")
}

func (l *racyMockLocker) Unlock(v string) error {
	if l.value == v {
		l.value = ""
		return nil
	}
	return fmt.Errorf("Couldn't unlock with provided value of %s, lock currently held by %s", v, l.value)
}

func uuidLocker() URILocker {
	localDynamoLocker, err := NewDynamoURILocker("label-mutex", "staging", fmt.Sprintf("%v", uuid.New()))
	if err != nil {
		panic(err)
	}
	return localDynamoLocker
}

var tests = []struct {
	eventFilename string
	eventName     string
	label         string
	issuesClient  issuesService
	uriLocker     URILocker
	err           bool
	locked        bool
	lockedOutput  string
}{
	{
		eventFilename: "testdata/pull_request.synchronize.json",
		eventName:     "pull_request",
		label:         "staging",
		issuesClient:  &happyPathLabelClient{},
		uriLocker:     &racyMockLocker{},
		err:           false,
		locked:        false,
		lockedOutput:  "false",
	},
	{
		eventFilename: "testdata/pull_request.labeled.json",
		eventName:     "pull_request",
		label:         "staging",
		issuesClient:  &happyPathLabelClient{},
		uriLocker:     &racyMockLocker{},
		err:           false,
		locked:        true,
		lockedOutput:  "true",
	},
	{
		eventFilename: "testdata/pull_request.labeled.json",
		eventName:     "pull_request",
		label:         "staging",
		issuesClient:  &happyPathLabelClient{},
		uriLocker:     uuidLocker(),
		err:           false,
		locked:        true,
		lockedOutput:  "true",
	},
}

func TestTable(t *testing.T) {
	for _, tt := range tests {
		t.Run(tt.eventFilename, func(t *testing.T) {
			event, err := ioutil.ReadFile(tt.eventFilename)
			if err != nil {
				t.Errorf("%s: %+v", tt.eventFilename, err)
				return
			}

			lm := &LabelMutex{
				context:      context.Background(),
				issuesClient: tt.issuesClient,
				uriLocker:    tt.uriLocker,
				event:        event,
				eventName:    "pull_request",
				label:        "staging",
			}
			err = lm.process()
			if !tt.err && err != nil {
				t.Errorf("%s: %+v", tt.eventFilename, err)
				return
			}
			if tt.err && err == nil {
				t.Errorf("%s: expected an error, didn't get one", tt.eventFilename)
				return
			}

			if lm.locked != tt.locked {
				t.Errorf("%s: locked: got %v, want %v", tt.eventFilename, lm.locked, tt.locked)
			}
			output := lm.output()
			if output["locked"] != tt.lockedOutput {
				t.Errorf("%s: output: got %v, want %v", tt.eventFilename, lm.locked, tt.locked)
			}
		})
	}
}
