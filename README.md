# label-mutex

> :warning: This is untested software. YMMV

A GitHub Action that facilitates obtaining and releasing a lock on a shared resource with PR labels. Add a label to obtain the lock, remove it or close/merge the PR to release the lock.

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

      - uses: urcomputeringpal/label-mutex@main
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
          LOCKED: ${{ steps.label-mutex.outputs.locked }}
        run: |
          if [ "$LOCKED" != "true" ]; then
            echo "::warning ::Couldn't obtain a lock on staging. Someone may already be using it: https://github.com/$GITHUB_REPOSITORY/deployments"
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

      - uses: urcomputeringpal/label-mutex@v0.0.1
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

## Lock table

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

### IAM

Ensure your AWS user has the following permissions on the above table:

```
dynamodb:GetItem
dynamodb:PutItem
dynamodb:DeleteItem
dynamodb:UpdateItem
```

## Acknowledgements

* https://github.com/sethvargo/go-hello-githubactions