package main

import (
	"context"
	"dagger/fp-go-net/internal/dagger"
)

type Repo struct{}

func (r *Repo) Test(ctx context.Context) error {
	_, err := dag.Container().
		From("golang:1.22").
		WithDirectory("/repo", dag.Host().Directory(".")).
		WithWorkdir("/repo").
		WithExec([]string{"go", "test", "./..."}).
		Sync(ctx)

	return err
}
