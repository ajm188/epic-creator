# epic-creator

Automatically create a set of JIRA issues under an epic.

## Usage

See `--help` for all usage and options.

First, you will need a go installation on your system.
[Follow these instructions](https://golang.org).

After installing go, you'll want to ensure any binaries you install via the go tooling are in your PATH:

```bash
$ export PATH=$GOPATH/bin:$PATH
```

### via `go install`

```bash
$ go install github.com/ajm188/epic-creator
$ epic-creator --help
```

### from source

You will need [glide](https://glide.sh) to install dependencies.

```bash
$ go install github.com/Masterminds/glide
```

```bash
$ git clone git@github.com:ajm188/epic-creator && cd epic-creator
$ glide install
$ go build .
$ ./epic-creator --help
```

## Inputs

### tickets.json

This file is a list of the issues to be created.

epic-creator expects the following schema:

```json
[
    {
        "project": "name-of-project",
        "params": {
            "key1": "val1",
            "key2": "val2",
            ...
        },
    }
    ...
]
```

The `params` field exists to pass arbitrary data to the templates for rendering the summary and description for the issue (see below).

### template files

You need two templates - one for the summary and one for the description.
Both will have access to the JSON payload of the issue being rendered (from tickets.json).
These templates should be written according to the spec in the [text/template](https://godoc.org/text/template) package.

See [summary.jira.tmpl](summary.jira.tmpl) and [description.jira.tmpl](description.jira.tmpl) for an example, as well as for some documentation around handling JIRA markup within the context of a Go template.
