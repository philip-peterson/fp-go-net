Introducing fp-go-net: to let you build FP-style networking applications in Go
====

In networking development, the problem frequently arises of testing your networked applications. Wiremock stubs are
suddenly something you have to worry about, among other things.

But with [ibm/fp-go](https://github.com/IBM/fp-go) and monadic programming, we can abstract away the statefulness of our programs
to test the logic alone, not the statefulness or networking aspects. This lets us avoid complicated network spying and allows us
to abstract our logic away from the statefulness.

Usage
---

### Installation

```
go get github.com/philip-peterson/fp-go-net
```

### Quick example — TCP echo server

```go
package main

import (
	"fmt"
	"net"

	. "github.com/IBM/fp-go/v2/function"
	IO "github.com/IBM/fp-go/v2/io"
	IOE "github.com/IBM/fp-go/v2/ioeither"

	fpnet "github.com/philip-peterson/fp-go-net"
)

var handler fpnet.Handler = func(c net.Conn) IOE.IOEither[fpnet.NetError, Void] {
	return Pipe2(
		fpnet.Read(1024)(c),
		IOE.Chain(func(b []byte) IOE.IOEither[fpnet.NetError, int] {
			return fpnet.Write(b)(c)
		}),
		IOE.Chain(func(_ int) IOE.IOEither[fpnet.NetError, Void] {
			return fpnet.Close(c)
		}),
	)
}

func main() {
	Pipe2(
		fpnet.Listen("tcp", ":9000"),
		IOE.Chain(fpnet.Serve(handler)),
		IOE.Fold(
			func(err fpnet.NetError) IO.IO[Void] {
				return func() Void { fmt.Println("error:", err); return VOID }
			},
			func(_ Void) IO.IO[Void] {
				return func() Void { return VOID }
			},
		),
	)()
}
```

All operations return `IOEither[NetError, A]` — a lazy value that, when executed (`()`), produces either a `NetError` (Left) or a success value (Right). Compose operations with `IOE.Chain` and fold the final result with `IOE.Fold`.

### TCP API

| Function | Description |
|---|---|
| `Listen(network, addr)` | Bind a stream listener |
| `Accept(listener)` | Accept the next incoming connection |
| `Dial(network, addr)` | Open an outbound connection |
| `Read(n)(conn)` | Read up to n bytes |
| `ReadFull(n)(conn)` | Read exactly n bytes |
| `ReadLine(conn)` | Read until newline |
| `Write(b)(conn)` | Write bytes |
| `Close(closer)` | Close a connection or listener |
| `Serve(handler)(listener)` | Accept connections in a loop, dispatching each to handler in a goroutine |

### UDP API

| Function | Description |
|---|---|
| `ListenPacket(network, addr)` | Bind a packet-oriented listener |
| `ReadFrom(n)(conn)` | Read one datagram (returns `Packet{Data, Addr}`) |
| `WriteTo(b, addr)(conn)` | Write a datagram to addr |

### Testing

The `github.com/philip-peterson/fp-go-net/testing` sub-package provides test helpers:

- `MockConn` — an in-memory `net.Conn`; inspect written bytes via `Written()`
- `AssertRight(t, result)` — fails the test if the Either is a Left

```go
import fpnettest "github.com/philip-peterson/fp-go-net/testing"

conn := &fpnettest.MockConn{}
result := fpnet.Write([]byte("hello"))(conn)()
fpnettest.AssertRight(t, result)
// conn.Written() == "hello"
```

Examples
---

Several examples are provided:

* [Simple HTTP server](https://github.com/philip-peterson/fp-go-net/blob/main/examples/webserver-helloworld/main.go)
* [IRC Server](https://github.com/philip-peterson/fp-go-net/blob/main/examples/ircserver/main.go)

Attribution
---

This library was largely developed with Claude. However, it is released under the MIT license because portions are handwritten and in the spirit of reserving no rights.
