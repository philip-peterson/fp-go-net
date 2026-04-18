package main

import (
	"context"
	"dagger/fp-go-net/internal/dagger"
)

type Repo struct{}

func (r *Repo) BuildNet(ctx context.Context) error {
	return r.build(ctx, "fp-go-net")
}

func (r *Repo) BuildTLS(ctx context.Context) error {
	return r.build(ctx, "fp-go-net-tls")
}

func (r *Repo) build(ctx context.Context, path string) error {
	_, err := dag.Container().
		From("golang:1.22").
		WithDirectory("/repo", dag.Host().Directory(".")).
		WithWorkdir("/repo/" + path).
		WithExec([]string{"go", "build", "./..."}).
		Sync(ctx)

	return err
}

func (r *Repo) TestIRC(ctx context.Context) error {
	_, err := dag.Container().
		From("golang:1.22").
		WithDirectory("/repo", dag.Host().Directory(".")).
		WithWorkdir("/repo/examples/ircserver").
		WithExec([]string{"go", "test", "./..."}).
		Sync(ctx)

	return err
}
