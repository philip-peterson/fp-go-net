Introducing fp-go-net: to let you build FP-style networking applications in Go
====

In networking development, the problem frequently arises of testing your networked applications. Wiremock stubs are
suddenly something you have to worry about, among other things.

But with [ibm/fp-go](https://github.com/IBM/fp-go) and monadic programming, we can abstract away the statefulness of our programs
to test the logic alone, not the statefulness or networking aspects. This lets us avoid complicated network spying and allows us
to abstract our logic away from the statefulness.

Examples
---

Several examples are provided:

* [Simple HTTP server](https://github.com/philip-peterson/fp-go-net/blob/main/examples/webserver-helloworld/main.go)
* [IRC Server](https://github.com/philip-peterson/fp-go-net/blob/main/examples/ircserver/main.go)

Attribution
---

This library was largely developed with Claude. However, it is released under the MIT license because portions are handwritten and in the spirit of reserving no rights.
