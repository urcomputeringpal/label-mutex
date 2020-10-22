package main

import (
	"context"
	"errors"
	"io/ioutil"
	"os"

	"github.com/google/go-github/v32/github"
	"github.com/hashicorp/go-multierror"
	"github.com/sethvargo/go-githubactions"
	"golang.org/x/oauth2"
)

func main() {
	c := &config{
		githubToken: githubactions.GetInput("GITHUB_TOKEN"),
		label:       githubactions.GetInput("label"),
		table:       githubactions.GetInput("table"),
		column:      githubactions.GetInput("column"),
		lock:        githubactions.GetInput("lock"),
	}
	err := c.Validate()
	if err != nil {
		githubactions.Fatalf("failed to validate input: %+v", err)
	}

	uriLocker, err := NewDynamoURILocker(c.table, c.column, c.lock)
	if err != nil {
		githubactions.Fatalf("failed to initialize: %+v", err)
	}

	event, err := ioutil.ReadFile(os.Getenv("GITHUB_EVENT_PATH"))
	if err != nil {
		githubactions.Fatalf("Couldn't read event: %+v", err)
	}

	ctx := context.Background()
	client := c.githubClient(ctx)

	labelMutex := &LabelMutex{
		context:            ctx,
		issuesClient:       client.Issues,
		pullRequestsClient: client.PullRequests,
		uriLocker:          uriLocker,
		label:              c.label,
		event:              event,
		eventName:          os.Getenv("GITHUB_EVENT_NAME"),
	}
	err = labelMutex.process()
	if err != nil {
		githubactions.Fatalf("error while processing event: %+v", err)
	}
}

type config struct {
	githubToken string
	label       string
	table       string
	column      string
	lock        string
}

func (c *config) Validate() error {
	var resultErr *multierror.Error
	if c.githubToken == "" {
		multierror.Append(resultErr, errors.New("input 'GITHUB_TOKEN' missing"))
	}
	if c.label == "" {
		multierror.Append(resultErr, errors.New("input 'label' missing"))
	}
	if c.table == "" {
		multierror.Append(resultErr, errors.New("input 'table' missing"))
	}
	if c.column == "" {
		multierror.Append(resultErr, errors.New("input 'column' missing"))
	}
	if c.lock == "" {
		multierror.Append(resultErr, errors.New("input 'lock' missing"))
	}
	return resultErr.ErrorOrNil()
}

func (c *config) githubClient(ctx context.Context) *github.Client {
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: c.githubToken},
	)
	tc := oauth2.NewClient(ctx, ts)
	return github.NewClient(tc)
}
