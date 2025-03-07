package main

import (
	"context"
	"dagger/dev/internal/dagger"
)

type Dev struct{}

func (m *Dev) BaseContainer(ctx context.Context,
	// +defaultPath="."
	src *dagger.Directory,
) *dagger.Container {
	goModCache := dag.CacheVolume("gomod")
	goBuildCache := dag.CacheVolume("gobuild")
	server := dag.Container().
		From("golang:1.21-alpine").
		WithMountedCache("/go/pkg/mod", goModCache).
		WithMountedCache("/root/.cache/go-build", goBuildCache).
		WithWorkdir("/app").
		WithFile("/app/go.mod", src.File("go.mod")).
		WithFile("/app/go.sum", src.File("go.sum")).
		WithExec([]string{"go", "mod", "download"}).
		WithDirectory("/app", src).
		WithEnvVariable("CGO_ENABLED", "0").
		WithExec([]string{"go", "build", "-ldflags", "-s -w", "-o", "server", "."}).
		File("server")

	return dag.
		Container().
		From("alpine:3.19").
		WithExposedPort(8080).
		WithFile("/server", server).
		WithWorkdir("/").
		WithExec([]string{"apk", "add", "--update", "--no-cache", "ca-certificates"}).
		WithEntrypoint([]string{"/server"})
}
