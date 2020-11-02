package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"

	"github.com/google/go-github/v32/github"
	"github.com/hashicorp/go-multierror"
	"github.com/wolfeidau/dynalock"
)

var (
	lockedSuffix = "locked"
)

type issuesService interface {
	AddLabelsToIssue(ctx context.Context, owner string, repo string, number int, labels []string) ([]*github.Label, *github.Response, error)
	RemoveLabelForIssue(ctx context.Context, owner string, repo string, number int, label string) (*github.Response, error)
}

type pullRequestService interface {
	List(ctx context.Context, owner string, repo string, opts *github.PullRequestListOptions) ([]*github.PullRequest, *github.Response, error)
}

// LabelMutex is a GitHub action that applies a label to exactly one pull request in your repository
type LabelMutex struct {
	issuesClient       issuesService
	pullRequestsClient pullRequestService
	context            context.Context
	uriLocker          URILocker
	event              []byte
	eventName          string
	label              string
	action             string
	pr                 *github.PullRequest
	locked             bool
	unlocked           bool
	htmlURL            string
}

func (lm *LabelMutex) output() map[string]string {
	output := make(map[string]string)
	if lm.locked {
		output["locked"] = "true"
	} else {
		output["locked"] = "false"
	}
	if lm.unlocked {
		output["unlocked"] = "true"
	} else {
		output["unlocked"] = "false"
	}
	if lm.htmlURL != "" {
		output["html_url"] = lm.htmlURL
	}
	return output
}

func (lm *LabelMutex) process() error {
	if lm.eventName == "pull_request" {
		return lm.processPR()
	}
	if lm.eventName == "push" {
		return lm.processPush()
	}
	return fmt.Errorf("Unknown event %s", lm.eventName)
}

func (lm *LabelMutex) processPush() error {
	var push github.PushEvent
	bytes := lm.clearJSONRepoOrgField(bytes.NewReader(lm.event))
	err := json.Unmarshal(bytes, &push)
	if err != nil {
		return err
	}
	value, err := lm.uriLocker.Read()
	if err == dynalock.ErrKeyNotFound {
		lm.locked = false
		lm.unlocked = true
	}
	if value == "" {
		lm.locked = false
		lm.unlocked = true
	} else {
		lm.htmlURL = value
		lm.locked = true
		lm.unlocked = false
	}
	return nil
}

func (lm *LabelMutex) clearJSONRepoOrgField(reader io.Reader) []byte {
	// workaround for https://github.com/google/go-github/issues/131
	var o map[string]interface{}
	dec := json.NewDecoder(reader)
	dec.UseNumber()
	dec.Decode(&o)
	if o != nil {
		repo := o["repository"]
		if repo != nil {
			if repo, ok := repo.(map[string]interface{}); ok {
				delete(repo, "organization")
			}
		}
	}
	b, _ := json.MarshalIndent(o, "", "  ")
	return b
}

func (lm *LabelMutex) processPR() error {
	var resultErr *multierror.Error
	var pr github.PullRequestEvent
	err := json.Unmarshal(lm.event, &pr)
	if err != nil {
		return err
	}
	lm.pr = pr.GetPullRequest()
	lm.action = pr.GetAction()

	var hasLockRequestLabel bool
	var hasLockConfirmedLabel bool
	for _, label := range lm.pr.Labels {
		if lm.label == label.GetName() {
			hasLockRequestLabel = true
		}
		if fmt.Sprintf("%s:%s", lm.label, lockedSuffix) == label.GetName() {
			hasLockConfirmedLabel = true
		}
	}

	var removedLabelName string
	var lockLabelRemoved bool
	if lm.action == "unlabeled" {
		removedLabelName = pr.GetLabel().GetName()
		if removedLabelName == lm.label {
			lockLabelRemoved = true
		}
	}

	lockValue := lm.pr.GetHTMLURL()
	if lm.pr.GetState() != "open" || lockLabelRemoved {
		log.Printf("Unlocking '%s' ...\n", lm.label)
		existing, err := lm.uriLocker.Unlock(lockValue)
		if err == nil || existing == "" {
			log.Println("Unlocked!")
			lm.locked = false
			lm.unlocked = true
		}
		if existing == "" {
			resultErr = multierror.Append(resultErr, err)
		} else {
			lm.locked = true
			lm.unlocked = false
			log.Printf("Lock '%s' currently claimed by %s  ...\n", lm.label, existing)
			lm.htmlURL = existing
		}

		resp, err := lm.issuesClient.RemoveLabelForIssue(lm.context, lm.pr.GetBase().Repo.Owner.GetLogin(), lm.pr.GetBase().Repo.GetName(), lm.pr.GetNumber(), lm.label)
		if resp.Response.StatusCode != http.StatusNotFound && err != nil {
			resultErr = multierror.Append(resultErr, err)
		}

		resp, err = lm.issuesClient.RemoveLabelForIssue(lm.context, lm.pr.GetBase().Repo.Owner.GetLogin(), lm.pr.GetBase().Repo.GetName(), lm.pr.GetNumber(), fmt.Sprintf("%s:%s", lm.label, lockedSuffix))
		if resp.Response.StatusCode != http.StatusNotFound && err != nil {
			resultErr = multierror.Append(resultErr, err)
		}

		return resultErr.ErrorOrNil()
	}

	if hasLockRequestLabel && hasLockConfirmedLabel {
		log.Printf("Lock '%s' should already be claimed by %s, confirming  ...\n", lm.label, lockValue)

		// double check
		success, existingValue, lockErr := lm.uriLocker.Lock(lockValue)
		if success {
			lm.locked = true
			log.Printf("Weird, the lock should have already been ours!")
			return nil
		}
		if existingValue == lockValue {
			lm.locked = true
			return nil
		}
		return lockErr
	}
	if hasLockRequestLabel && !hasLockConfirmedLabel {
		log.Printf("Lock '%s' requested but not confirmed, trying to lock with %s  ...\n", lm.label, lockValue)
		success, existingValue, lockErr := lm.uriLocker.Lock(lockValue)
		if lockErr != nil {
			return lockErr
		}
		if success {
			log.Printf("Lock '%s' obtained\n", lm.label)
			lm.locked = true
			lm.htmlURL = lockValue
			labelsToAdd := []string{fmt.Sprintf("%s:%s", lm.label, lockedSuffix)}
			_, _, err := lm.issuesClient.AddLabelsToIssue(lm.context, lm.pr.GetBase().Repo.Owner.GetLogin(), lm.pr.GetBase().Repo.GetName(), lm.pr.GetNumber(), labelsToAdd)
			if err != nil {
				return err
			}
			return nil
		}
		if existingValue != "" {
			log.Printf("Lock '%s' claimed by %s\n", lm.label, existingValue)
			lm.locked = true
			lm.htmlURL = existingValue
			return nil
		}
		return errors.New("Unknown error")
	}

	log.Printf("Label '%s' not present, doing nothing\n", lm.label)
	return resultErr.ErrorOrNil()
}
