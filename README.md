p2
==

This repository contains the starter code for project 2 (15-440, Spring 2014). These instructions
assume you have set your `GOPATH` to point to the repository's root `p2/` directory.

This project was designed for, and tested on AFS cluster machines, though you may choose to
write and build your code locally as well.

## Instructions

### Building & Compiling Your Code

More detailed instructions TBA.

If at any point you have any trouble with building, installing, or testing your code, the article
titled [How to Write Go Code](http://golang.org/doc/code.html) is a great resource for understanding
how Go workspaces are built and organized. You might also find the documentation for the
[`go` command](http://golang.org/cmd/go/) to be helpful. As always, feel free to post your questions
on Piazza.

### Running the tests

TBA.

### Submitting to Autolab

TBA.

## Miscellaneous

### Reading the Starter Code Documentation

Before you begin the project, you should read and understand all of the starter code we provide.
To make this experience a little less traumatic, fire up a web server and read the
documentation in a browser by executing the following command:

```sh
godoc -http=:6060 &
```

Then, navigate to [localhost:6060/pkg/github.com/cmu440](http://localhost:6060/pkg/github.com/cmu440)
in a browser (note that you can execute this command from anywhere in your system, assuming your `GOPATH`
is set correctly).

### Using Go on AFS

For those students who wish to write their Go code on AFS (either in a cluster or remotely), you will
need to set the `GOROOT` environment variable as follows (this is required because Go is installed
in a custom location on AFS machines):

```bash
export GOROOT=/usr/local/lib/go
```
