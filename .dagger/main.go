package main

import (
	"context"

	"dagger/fp-go-net/internal/dagger"
)

type FpGoNet struct{}

func (r *FpGoNet) Ci(ctx context.Context,
	// +defaultPath="."
	src *dagger.Directory,
) error {
	if err := r.buildNet(ctx, src); err != nil {
		return err
	}
	if err := r.buildTLS(ctx, src); err != nil {
		return err
	}
	if err := r.testIRC(ctx, src); err != nil {
		return err
	}
	return nil
}

func (r *FpGoNet) buildNet(ctx context.Context, src *dagger.Directory) error {
	return r.build(ctx, src, "fp-go-net")
}

func (r *FpGoNet) buildTLS(ctx context.Context, src *dagger.Directory) error {
	return r.build(ctx, src, "fp-go-net-tls")
}

func (r *FpGoNet) build(ctx context.Context, src *dagger.Directory, path string) error {
	_, err := dag.Container().
		From("golang:1.22").
		WithDirectory("/repo", src).
		WithWorkdir("/repo/" + path).
		WithExec([]string{"go", "build", "./..."}).
		Sync(ctx)

	return err
}

func (r *FpGoNet) testIRC(ctx context.Context, src *dagger.Directory) error {
	_, err := dag.Container().
		From("golang:1.22").
		WithDirectory("/repo", src).
		WithWorkdir("/repo/examples/ircserver").
		WithExec([]string{"go", "test", "./..."}).
		Sync(ctx)

	return err
}
