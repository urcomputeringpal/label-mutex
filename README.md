### Labeled / Opened / Synchronize

- If labels contain `$label`
    - Attempt to obtain lock for `$label` with `html_url` as data
        - If lock obtained:
            - Add `$label:locked`
            - Remove `$label:pending`
        - If not:
            - Remove `$label`
            - Add `$label:pending`
            - Facilitate communication between lock holder and current actor
    - Loop until appropriate labels show up in a get call

### Unlabeled / Closed

- Read lock 
    - If lock data matches `html_url`
        - Unlock
        - Remove
- Find all PRs matching `$label:pending`
    - Choose one??
    - Label it with `$label` to kick off the above

## Acknowledgements

* https://github.com/sethvargo/go-hello-githubactions