# deploywatch

Watch AWS CodeDeploy deployment statuses in real-time from the console.

AWS credentials must be present in environment.

## Install

Download the [latest release](https://github.com/atongen/deploywatch/releases), extract it,
and put it somewhere on your PATH.

or

```sh
$ go get github.com/atongen/deploywatch
```

or

```sh
$ mkdir -p $GOPATH/src/github.com/atongen
$ cd $GOPATH/src/github.com/atongen
$ git clone https://github.com/atongen/deploywatch
$ cd deploywatch
$ go install
```

## Testing

[wip]

```sh
$ cd $GOPATH/src/github.com/atongen/deploywatch
$ go test -cover
```

## Releases

```sh
$ mkdir -p $GOPATH/src/github.com/atongen
$ cd $GOPATH/src/github.com/atongen
$ git clone git@github.com:atongen/deploywatch.git
$ cd deploywatch
$ make release
```

## Command-Line Options

```
λ deploywatch [OPTIONS] DEPLOY_ID [DEPLOY_ID]...
Options:
  -compact
        Print compact output
  -groups string
        CodeDeploy deployment groups csv (optional)
  -name string
        CodeDeploy application name (optional)
  -version
        Print version information and exit
```
