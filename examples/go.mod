module github.com/philip-peterson/fp-go-net-examples

go 1.26

require (
	github.com/IBM/fp-go/v2 v2.2.71
	github.com/philip-peterson/fp-go-net v1.4.0
	github.com/philip-peterson/fp-go-net-tls v1.4.0
)

replace (
	github.com/philip-peterson/fp-go-net => ..
	github.com/philip-peterson/fp-go-net-tls => ../fp-go-net-tls
)
