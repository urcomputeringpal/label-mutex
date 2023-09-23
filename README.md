# label-mutex

A GitHub Action that facilitates obtaining, releasing, and representing a lock on a shared resource using PR labels. Add a label to obtain the lock, remove it or close/merge the PR to release the lock.

Let's say you'd like to allow engineers to deploy PRs to staging by adding a `staging` label to their PRs, but want to ensure that only one PR can be deployed to staging at a time. This action can be used to ensure that only one PR has a `staging` label at the same time like so:

### Confirm lock when labeled

```yaml
on:
  pull_request:
    types:
      - opened
      - labeled
      - synchronize
      - reopened

      #jobs:
      #  deploy:
      #    steps:

      - uses: urcomputeringpal/label-mutex@v0.3.0
        id: label-mutex
        env:
          AWS_DEFAULT_REGION: us-east-1
          AWS_ACCESS_KEY_ID: ${{ secrets.AWS_ACCESS_KEY_ID }}
          AWS_SECRET_ACCESS_KEY: ${{ secrets.AWS_SECRET_ACCESS_KEY }}
        with:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
          label: staging
          lock: staging
      - name: fail-if-not-locked
        env:
          PR_URL: ${{ github.event.pull_request.html_url }}
          LOCKED: ${{ steps.label-mutex.outputs.locked }}
          LOCK_URL: ${{ steps.label-mutex.outputs.html_url }}
        run: |
          if [ "$LOCKED" == "true" ] && [ $PR_URL" != "$LOCK_URL" ]; then
            echo "::warning ::Couldn't obtain a lock on staging. Someone may already be using it: $LOCK_URL"
            exit 1
          fi
```

### Unlock on unlabel and close

```yaml
on:
  pull_request:
    types:
      - closed
      - unlabeled

      #jobs:
      #  unlock:
      #    steps:

      - uses: urcomputeringpal/label-mutex@v0.3.0
        id: label-mutex
        env:
          AWS_DEFAULT_REGION: us-east-1
          AWS_ACCESS_KEY_ID: ${{ secrets.AWS_ACCESS_KEY_ID }}
          AWS_SECRET_ACCESS_KEY: ${{ secrets.AWS_SECRET_ACCESS_KEY }}
        with:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
          label: staging
          lock: staging
```

## Setup

### AWS

#### Lock table

```hcl
resource "aws_dynamodb_table" "label_mutex" {
  name         = "label-mutex"
  billing_mode = "PAY_PER_REQUEST"
  hash_key     = "name"
  range_key    = "id"

  attribute {
    name = "id"
    type = "S"
  }

  attribute {
    name = "name"
    type = "S"
  }

  ttl {
    attribute_name = "expires"
    enabled        = true
  }
}
```

#### IAM

Ensure your AWS user has the following permissions on the above table:

```
dynamodb:GetItem
dynamodb:PutItem
dynamodb:DeleteItem
dynamodb:UpdateItem
```

## GCS

- Setup a new project at the [Google APIs Console](https://console.developers.google.com/) and enable the Cloud Storage API.
- Install the [Google Cloud SDK tool](https://cloud.google.com/sdk/downloads) and configure your project and your OAuth credentials.
- Create a bucket in which to store your lock file using the command `gsutil mb gs://your-bucket-name`.
- Enable object versioning in your bucket using the command `gsutil versioning set on gs://your-bucket-name`.

## Acknowledgements

- https://github.com/sethvargo/go-hello-githubactions
