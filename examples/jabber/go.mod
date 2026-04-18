module github.com/philip-peterson/fp-go-net-jabber-example

go 1.26

require (
	github.com/IBM/fp-go/v2 v2.2.71
	github.com/philip-peterson/fp-go-net v0.0.0
	github.com/philip-peterson/fp-go-net-tls v0.0.0
)

replace (
	github.com/philip-peterson/fp-go-net => ../../
	github.com/philip-peterson/fp-go-net-tls => ../../tls
)
