package main

import (
	"context"
	"errors"
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
	return false, l.value, errors.New("some imaginary lock conflict error")
}

func (l *racyMockLocker) Unlock() error {
	l.value = ""
	return nil
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
