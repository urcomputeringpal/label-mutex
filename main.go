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
		partition:   githubactions.GetInput("partition"),
		bucket:      githubactions.GetInput("bucket"),
		lock:        githubactions.GetInput("lock"),
	}
	err := c.Validate()
	if err != nil {
		githubactions.Fatalf("failed to validate input: %+v", err)
	}

	var uriLocker URILocker
	var initErr error
	if c.bucket == "" {
		uriLocker, initErr = NewDynamoURILocker(c.table, c.partition, c.lock)
	} else {
		uriLocker, initErr = NewGCSLocker(c.bucket, c.lock)
	}
	if initErr != nil {
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
	output := labelMutex.output()
	for k, v := range output {
		githubactions.SetOutput(k, v)
	}
}

type config struct {
	githubToken string
	label       string
	table       string
	partition   string
	bucket      string
	lock        string
}

func (c *config) Validate() error {
	var resultErr *multierror.Error
	if c.githubToken == "" {
		resultErr = multierror.Append(resultErr, errors.New("input 'GITHUB_TOKEN' missing"))
	}
	if c.label == "" {
		resultErr = multierror.Append(resultErr, errors.New("input 'label' missing"))
	}
	if c.table == "" && c.bucket == "" {
		resultErr = multierror.Append(resultErr, errors.New("either 'table' or 'bucket' is required"))
	}
	if c.partition == "" {
		c.partition = c.bucket
	}
	if c.partition == "" {
		resultErr = multierror.Append(resultErr, errors.New("input 'partition' missing"))
	}
	if c.lock == "" {
		resultErr = multierror.Append(resultErr, errors.New("input 'lock' missing"))
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
