package main

import (
	"context"
	"encoding/json"
	"fmt"

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

func (lm *LabelMutex) process() error {
	githubactions.Warningf("processing")
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
	if lm.pr.GetState() != "open" {
		githubactions.Warningf("removing lock")
		err = lm.uriLocker.Unlock()
		multierror.Append(resultErr, err)

		_, err = lm.issuesClient.RemoveLabelForIssue(lm.context, lm.pr.GetBase().Repo.Owner.GetLogin(), lm.pr.GetBase().Repo.GetName(), lm.pr.GetNumber(), lm.label)
		multierror.Append(resultErr, err)

		_, err = lm.issuesClient.RemoveLabelForIssue(lm.context, lm.pr.GetBase().Repo.Owner.GetLogin(), lm.pr.GetBase().Repo.GetName(), lm.pr.GetNumber(), fmt.Sprintf("%s:%s", lm.label, lockedSuffix))
		multierror.Append(resultErr, err)

		return resultErr.ErrorOrNil()
	}

	if hasLockRequestLabel && hasLockConfirmedLabel {
		githubactions.Warningf("double checking lock")
		// double check
		success, existingValue, lockErr := lm.uriLocker.Lock(lm.pr.GetHTMLURL())
		if success {
			lm.locked = true
			githubactions.Warningf("weird, the lock should have already been obtained!")
			return nil
		}
		if existingValue == lm.pr.GetHTMLURL() {
			lm.locked = true
			return nil
		}
		return lockErr
	}
	if hasLockRequestLabel && !hasLockConfirmedLabel {
		githubactions.Warningf("trying to obtain lock")
		success, existingValue, lockErr := lm.uriLocker.Lock(lm.pr.GetHTMLURL())
		if success {
			lm.locked = true
			labelsToAdd := []string{fmt.Sprintf("%s:%s", lm.label, lockedSuffix)}
			_, _, err := lm.issuesClient.AddLabelsToIssue(lm.context, lm.pr.GetBase().Repo.Owner.GetLogin(), lm.pr.GetBase().Repo.GetName(), lm.pr.GetNumber(), labelsToAdd)
			if err != nil {
				return err
			}
			return nil
		}
		if existingValue != "" {
			lm.lockOwner = existingValue
			return nil
		}
		if lockErr != nil {
			return lockErr
		}
	}

	return resultErr.ErrorOrNil()
}
