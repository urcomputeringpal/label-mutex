package main

import (
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"testing"

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

func TestNoop(t *testing.T) {
	event, err := ioutil.ReadFile("testdata/pull_request.synchronize.json")
	lm := &LabelMutex{
		context:      context.Background(),
		issuesClient: &happyPathLabelClient{},
		uriLocker:    &racyMockLocker{},
		event:        event,
		eventName:    "pull_request",
		label:        "staging",
	}
	if err != nil {
		t.Fatal(err)
	}

	if err = lm.process(); err != nil {
		t.Fatal(err)
	}

	if lm.action != "synchronize" {
		t.Fatalf("Expected action to be synchronize: %+v", lm.action)
	}

	if lm.pr.GetBase().Repo.Owner.GetLogin() != "urcomputeringpal" {
		t.Fatalf("Expected org to be urcomputeringpal: %+v", lm.pr)
	}

	if lm.locked {
		t.Fatalf("Expected lock to not have been obtained: %+v", lm)
	}

}

func TestLabeled(t *testing.T) {
	event, err := ioutil.ReadFile("testdata/pull_request.labeled.json")
	lm := &LabelMutex{
		context:      context.Background(),
		issuesClient: &happyPathLabelClient{},
		uriLocker:    &racyMockLocker{},
		event:        event,
		eventName:    "pull_request",
		label:        "staging",
	}
	if err != nil {
		t.Fatal(err)
	}

	if err = lm.process(); err != nil {
		t.Fatal(err)
	}

	if lm.action != "labeled" {
		t.Fatalf("Expected action to be labeled: %+v", lm.action)
	}

	if lm.pr.GetHTMLURL() != "https://github.com/urcomputeringpal/label-mutex/pull/1" {
		t.Fatalf("Expected GetHTMLURL to be a url: %+v", lm.pr.GetHTMLURL())
	}

	if !lm.locked {
		t.Fatalf("Expected lock to have been obtained: %+v", lm)
	}

}

func TestLabeledDynamo(t *testing.T) {
	localDynamoLocker, err := NewDynamoURILocker("label-mutex", "staging", "staging")
	if err != nil {
		t.Fatalf("failed to initialize: %+v", err)
	}

	event, err := ioutil.ReadFile("testdata/pull_request.labeled.json")
	lm := &LabelMutex{
		context:      context.Background(),
		issuesClient: &happyPathLabelClient{},
		uriLocker:    localDynamoLocker,
		event:        event,
		eventName:    "pull_request",
		label:        "staging",
	}
	if err != nil {
		t.Fatal(err)
	}

	if err = lm.process(); err != nil {
		t.Fatal(err)
	}

	if lm.action != "labeled" {
		t.Fatalf("Expected action to be labeled: %+v", lm.action)
	}

	if lm.pr.GetHTMLURL() != "https://github.com/urcomputeringpal/label-mutex/pull/1" {
		t.Fatalf("Expected GetHTMLURL to be a url: %+v", lm.pr.GetHTMLURL())
	}

	if !lm.locked {
		t.Fatalf("Expected lock to have been obtained: %+v", lm)
	}

}
