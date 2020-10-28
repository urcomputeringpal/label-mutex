package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"

	"github.com/google/go-github/v32/github"
	"github.com/hashicorp/go-multierror"
	"github.com/sethvargo/go-githubactions"
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
	lockOwner          string
}

func (lm *LabelMutex) output() map[string]string {
	output := make(map[string]string)
	if lm.locked {
		output["locked"] = "true"
	} else {
		output["locked"] = "false"
	}
	return output
}

func (lm *LabelMutex) process() error {
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
	lockValue := lm.pr.GetHTMLURL()
	if lm.pr.GetState() != "open" {
		err = lm.uriLocker.Unlock(lockValue)
		if err == nil {
			lm.locked = false
		}
		resultErr = multierror.Append(resultErr, err)

		_, err = lm.issuesClient.RemoveLabelForIssue(lm.context, lm.pr.GetBase().Repo.Owner.GetLogin(), lm.pr.GetBase().Repo.GetName(), lm.pr.GetNumber(), lm.label)
		resultErr = multierror.Append(resultErr, err)

		_, err = lm.issuesClient.RemoveLabelForIssue(lm.context, lm.pr.GetBase().Repo.Owner.GetLogin(), lm.pr.GetBase().Repo.GetName(), lm.pr.GetNumber(), fmt.Sprintf("%s:%s", lm.label, lockedSuffix))
		resultErr = multierror.Append(resultErr, err)

		return resultErr.ErrorOrNil()
	}

	if hasLockRequestLabel && hasLockConfirmedLabel {
		log.Printf("Lock '%s' should already be owned by %s, confirming  ...\n", lm.label, lockValue)

		// double check
		success, existingValue, lockErr := lm.uriLocker.Lock(lockValue)
		if success {
			lm.locked = true
			githubactions.Warningf("weird, the lock should have already been ours!")
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
			labelsToAdd := []string{fmt.Sprintf("%s:%s", lm.label, lockedSuffix)}
			_, _, err := lm.issuesClient.AddLabelsToIssue(lm.context, lm.pr.GetBase().Repo.Owner.GetLogin(), lm.pr.GetBase().Repo.GetName(), lm.pr.GetNumber(), labelsToAdd)
			if err != nil {
				return err
			}
			return nil
		}
		if existingValue != "" {
			log.Printf("Lock '%s' already claimed by %s  ...\n", lm.label, existingValue)
			lm.lockOwner = existingValue
			return nil
		}
		return errors.New("Unknown error")
	}

	log.Printf("Label '%s' not present, doing nothing.\n", lm.label)
	return resultErr.ErrorOrNil()
}
