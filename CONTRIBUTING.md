# Contributing

We welcome your help building the M3 Operator.

## Getting Started

The M3 Operator uses `dep` to manage dependencies. To get started:

```shell
git submodule update --init --recursive
make install-tools
```

## Making A Change

-   Before making any significant changes, please [open an issue](https://github.com/m3db/m3db-operator/issues).
-   Discussing your proposed changes ahead of time maked the contribution process smoother for everyone.

Once the changes are discussed and you have your code ready, make sure that tests are passing:

```bash
make test-all
```

Your pull request is most likely to be accepted if it:

-   Includes tests for new functionality.
-   Follows the guidelines in [Effective Go](https://golang.org/doc/effective_go.html) and the [Go team's common code
    review comments](https://github.com/golang/go/wiki/CodeReviewComments).
-   Has a [good commit message](http://tbaggery.com/2008/04/19/a-note-about-git-commit-messages.html).
