package main

import (
	"context"
	"strings"

	"dagger/fp-go-net/internal/dagger"
)

type FpGoNet struct{}

func (r *FpGoNet) Ci(ctx context.Context,
	// +defaultPath="."
	src *dagger.Directory,
) error {
	if err := r.BuildNet(ctx, src); err != nil {
		return err
	}
	if err := r.BuildTLS(ctx, src); err != nil {
		return err
	}
	if err := r.TestIRC(ctx, src); err != nil {
		return err
	}
	return nil
}

func (r *FpGoNet) BuildNet(ctx context.Context, src *dagger.Directory) error {
	return r.build(ctx, src, ".")
}

func (r *FpGoNet) BuildTLS(ctx context.Context, src *dagger.Directory) error {
	return r.build(ctx, src, "fp-go-net-tls")
}

var baseContainer = dag.Container().From("golang:1.26")

func (r *FpGoNet) build(ctx context.Context, src *dagger.Directory, path string) error {
	_, err := baseContainer.
		WithDirectory("/repo", src).
		WithWorkdir("/repo/" + path).
		WithExec([]string{"go", "build", "./..."}).
		Sync(ctx)

	return err
}

// BumpMinor tags the next minor version and pushes it. Returns the new tag.
func (r *FpGoNet) BumpMinor(ctx context.Context,
	// +defaultPath="."
	src *dagger.Directory,
	token *dagger.Secret,
) (string, error) {
	script := strings.Join([]string{
		`LATEST=$(git tag --list 'v*' --sort=-v:refname | grep -v '/' | head -1)`,
		`LATEST=${LATEST:-v0.0.0}`,
		`MAJOR=$(echo "$LATEST" | sed 's/v\([0-9]*\)\..*/\1/')`,
		`MINOR=$(echo "$LATEST" | sed 's/v[0-9]*\.\([0-9]*\)\..*/\1/')`,
		`NEW_TAG="v${MAJOR}.$((MINOR + 1)).0"`,
		`TLS_TAG="fp-go-net-tls/${NEW_TAG}"`,
		`sed -i "s|github.com/philip-peterson/fp-go-net v[0-9.]*|github.com/philip-peterson/fp-go-net ${NEW_TAG}|g" fp-go-net-tls/go.mod examples/go.mod`,
		`sed -i "s|github.com/philip-peterson/fp-go-net/fp-go-net-tls v[0-9.]*|github.com/philip-peterson/fp-go-net/fp-go-net-tls ${NEW_TAG}|g" examples/go.mod`,
		`REMOTE=$(git remote get-url origin)`,
		// convert git@github.com:owner/repo.git → https://owner/repo.git
		`HTTPS=$(echo "$REMOTE" | sed 's|git@github.com:|https://github.com/|')`,
		`AUTH_URL=$(echo "$HTTPS" | sed "s|https://github.com/|https://x-access-token:${GIT_TOKEN}@github.com/|")`,
		`git config user.email "ci@dagger"`,
		`git config user.name "Dagger CI"`,
		`git add fp-go-net-tls/go.mod examples/go.mod`,
		`git commit -m "chore: release ${NEW_TAG}"`,
		`BRANCH=$(git rev-parse --abbrev-ref HEAD)`,
		`git tag "$NEW_TAG"`,
		`git tag "$TLS_TAG"`,
		`git push "$AUTH_URL" "HEAD:${BRANCH}" "$NEW_TAG" "$TLS_TAG"`,
		`echo "$NEW_TAG"`,
	}, "\n")

	return dag.Container().From("golang:1.26").
		WithDirectory("/repo", src).
		WithWorkdir("/repo").
		WithSecretVariable("GIT_TOKEN", token).
		WithExec([]string{"sh", "-c", script}).
		Stdout(ctx)
}

func (r *FpGoNet) TestIRC(ctx context.Context, src *dagger.Directory) error {
	_, err := baseContainer.
		WithDirectory("/repo", src).
		WithWorkdir("/repo/examples/ircserver").
		WithExec([]string{"go", "test", "./..."}).
		Sync(ctx)

	return err
}
