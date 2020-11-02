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

func (l *racyMockLocker) Unlock(v string) (string, error) {
	if l.value == v {
		l.value = ""
		return "", nil
	}
	return l.value, fmt.Errorf("Couldn't unlock with provided value of %s, lock currently held by %s", v, l.value)
}

func (l *racyMockLocker) Read() (string, error) {
	return l.value, nil
}

func uuidLocker() URILocker {
	localDynamoLocker, err := NewDynamoURILocker("label-mutex", "staging", fmt.Sprintf("%v", uuid.New()))
	if err != nil {
		panic(err)
	}
	return localDynamoLocker
}

type labelMutexTest struct {
	eventFilename  string
	eventName      string
	label          string
	issuesClient   issuesService
	uriLocker      URILocker
	err            bool
	locked         bool
	lockedOutput   string
	unlockedOutput string
	htmlURLOutput  string
}

var URILockerOne URILocker
var URILockerTwo URILocker
var tests []labelMutexTest

func init() {
	URILockerOne = uuidLocker()
	URILockerTwo = uuidLocker()
	tests = []labelMutexTest{
		// try to read it
		{
			eventFilename:  "testdata/push.json",
			eventName:      "push",
			label:          "staging",
			issuesClient:   &happyPathLabelClient{},
			uriLocker:      URILockerOne,
			err:            false,
			locked:         false,
			lockedOutput:   "false",
			unlockedOutput: "true",
			htmlURLOutput:  "",
		},
		{
			eventFilename:  "testdata/1/pull_request.synchronize.json",
			eventName:      "pull_request",
			label:          "staging",
			issuesClient:   &happyPathLabelClient{},
			uriLocker:      &racyMockLocker{},
			err:            false,
			locked:         false,
			lockedOutput:   "false",
			unlockedOutput: "false",
			htmlURLOutput:  "",
		},
		{
			eventFilename:  "testdata/1/pull_request.labeled.json",
			eventName:      "pull_request",
			label:          "staging",
			issuesClient:   &happyPathLabelClient{},
			uriLocker:      &racyMockLocker{},
			err:            false,
			locked:         true,
			lockedOutput:   "true",
			unlockedOutput: "false",
			htmlURLOutput:  "https://github.com/urcomputeringpal/label-mutex/pull/1",
		},
		// add the label and obtain the lock
		{
			eventFilename:  "testdata/1/pull_request.labeled.json",
			eventName:      "pull_request",
			label:          "staging",
			issuesClient:   &happyPathLabelClient{},
			uriLocker:      URILockerOne,
			err:            false,
			locked:         true,
			lockedOutput:   "true",
			unlockedOutput: "false",
			htmlURLOutput:  "https://github.com/urcomputeringpal/label-mutex/pull/1",
		},
		// add the label again
		{
			eventFilename:  "testdata/1/pull_request.labeled.json",
			eventName:      "pull_request",
			label:          "staging",
			issuesClient:   &happyPathLabelClient{},
			uriLocker:      URILockerOne,
			err:            false,
			locked:         true,
			lockedOutput:   "true",
			unlockedOutput: "false",
			htmlURLOutput:  "https://github.com/urcomputeringpal/label-mutex/pull/1",
		},
		// try to clobber it
		{
			eventFilename:  "testdata/2/pull_request.labeled.json",
			eventName:      "pull_request",
			label:          "staging",
			issuesClient:   &happyPathLabelClient{},
			uriLocker:      URILockerOne,
			err:            false,
			locked:         true,
			lockedOutput:   "true",
			unlockedOutput: "false",
			htmlURLOutput:  "https://github.com/urcomputeringpal/label-mutex/pull/1",
		},
		// try to read it
		{
			eventFilename:  "testdata/push.json",
			eventName:      "push",
			label:          "staging",
			issuesClient:   &happyPathLabelClient{},
			uriLocker:      URILockerOne,
			err:            false,
			locked:         true,
			lockedOutput:   "true",
			unlockedOutput: "false",
			htmlURLOutput:  "https://github.com/urcomputeringpal/label-mutex/pull/1",
		},
		// try to remove it from a PR that doesn't have it
		{
			eventFilename:  "testdata/2/pull_request.unlabeled.json",
			eventName:      "pull_request",
			label:          "staging",
			issuesClient:   &happyPathLabelClient{},
			uriLocker:      URILockerOne,
			err:            false,
			locked:         true,
			lockedOutput:   "true",
			unlockedOutput: "false",
			htmlURLOutput:  "https://github.com/urcomputeringpal/label-mutex/pull/1",
		},
		// close to remove the first lock
		{
			eventFilename:  "testdata/1/pull_request.closed.json",
			eventName:      "pull_request",
			label:          "staging",
			issuesClient:   &happyPathLabelClient{},
			uriLocker:      URILockerOne,
			err:            false,
			locked:         false,
			lockedOutput:   "false",
			unlockedOutput: "true",
			htmlURLOutput:  "",
		},
		{
			eventFilename:  "testdata/1/pull_request.labeled.json",
			eventName:      "pull_request",
			label:          "staging",
			issuesClient:   &happyPathLabelClient{},
			uriLocker:      URILockerTwo,
			err:            false,
			locked:         true,
			lockedOutput:   "true",
			unlockedOutput: "false",
			htmlURLOutput:  "https://github.com/urcomputeringpal/label-mutex/pull/1",
		},
		{
			eventFilename:  "testdata/1/pull_request.unlabeled.json",
			eventName:      "pull_request",
			label:          "staging",
			issuesClient:   &happyPathLabelClient{},
			uriLocker:      URILockerTwo,
			err:            false,
			locked:         false,
			lockedOutput:   "false",
			unlockedOutput: "true",
			htmlURLOutput:  "",
		},
	}
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
				eventName:    tt.eventName,
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
				t.Errorf("%s: outputs.locked: got %v, want %v", tt.eventFilename, output["locked"], tt.lockedOutput)
			}
			if output["unlocked"] != tt.unlockedOutput {
				t.Errorf("%s: outputs.unlocked: got %v, want %v", tt.eventFilename, output["unlocked"], tt.unlockedOutput)
			}
			if output["html_url"] != tt.htmlURLOutput {
				t.Errorf("%s: outputs.html_url: got %v, want %v", tt.eventFilename, output["html_url"], tt.htmlURLOutput)
			}
		})
	}
}
