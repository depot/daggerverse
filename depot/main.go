package main

import (
	"context"
	"strings"
)

const (
	DefaultDockerHost = "unix:///var/run/docker.sock"
)

type Depot struct {
	// DockerHost is used for --load.
	DockerHost string

	Token      *Secret
	Project    string
	Directory  *Directory
	Dockerfile string

	Platforms []Platform

	Push    bool
	Load    bool
	SBOM    bool
	Lint    bool
	NoCache bool

	Tags      []string
	BuildArgs []string
	Labels    []string
	Outputs   []string

	Provenance string
}

func (m *Depot) WithToken(token *Secret) *Depot {
	m.Token = token
	return m
}

func (m *Depot) WithProject(project string) *Depot {
	m.Project = project
	return m
}

func (m *Depot) WithDirectory(directory *Directory) *Depot {
	m.Directory = directory
	return m
}

func (m *Depot) WithDockerfile(dockerfile string) *Depot {
	m.Dockerfile = dockerfile
	return m
}

func (m *Depot) WithNoCache() *Depot {
	m.NoCache = true
	return m
}

func (m *Depot) WithPush() *Depot {
	m.Push = true
	return m
}

func (m *Depot) WithLoad() *Depot {
	m.Load = true
	return m
}

func (m *Depot) WithSBOM() *Depot {
	m.SBOM = true
	return m
}

func (m *Depot) WithPlatform(platform Platform) *Depot {
	m.Platforms = append(m.Platforms, platform)
	return m
}

func (m *Depot) WithTag(tag string) *Depot {
	m.Tags = append(m.Tags, tag)
	return m
}

func (m *Depot) WithBuildArg(arg string) *Depot {
	m.BuildArgs = append(m.BuildArgs, arg)
	return m
}

func (m *Depot) WithLabel(label string) *Depot {
	m.Labels = append(m.Labels, label)
	return m
}

func (m *Depot) WithOutput(output string) *Depot {
	m.Outputs = append(m.Outputs, output)
	return m
}

func (m *Depot) Run(ctx context.Context) (*Container, error) {
	return build(
		ctx,
		m.Token,
		m.Project,
		m.Directory,
		m.Dockerfile,
		m.Platforms,
		m.Push,
		m.DockerHost,
		m.Load,
		m.SBOM,
		m.NoCache,
		m.Tags,
		m.BuildArgs,
		m.Labels,
		m.Outputs,
		m.Provenance,
	)
}

// example usage: "dagger call depot build --token hunter2 --project 1234 --directory . platform --arg linux/arm64"
func (m *Depot) Build(ctx context.Context,
	token *Secret,
	project string,
	directory *Directory,
	dockerfile Optional[string],
	platforms Optional[[]Platform],
	push Optional[bool],
	// docker host (default: unix:///var/run/docker.sock)
	dockerHost Optional[string],
	load Optional[bool],
	sbom Optional[bool],
	noCache Optional[bool],
	tags Optional[[]string],
	buildArgs Optional[[]string],
	labels Optional[[]string],
	outputs Optional[[]string],
	provenance Optional[string],
) (*Container, error) {
	return build(
		ctx,
		token,
		project,
		directory,
		dockerfile.GetOr(m.Dockerfile),
		platforms.GetOr(m.Platforms),
		push.GetOr(m.Push),
		dockerHost.GetOr(m.DockerHost),
		load.GetOr(m.Load),
		sbom.GetOr(m.SBOM),
		noCache.GetOr(m.NoCache),
		tags.GetOr(m.Tags),
		buildArgs.GetOr(m.BuildArgs),
		labels.GetOr(m.Labels),
		outputs.GetOr(m.Outputs),
		provenance.GetOr(m.Provenance),
	)
}

func build(ctx context.Context,
	token *Secret,
	project string,
	directory *Directory,
	dockerfile string,
	platforms []Platform,
	push bool,
	dockerHost string,
	load bool,
	sbom bool,
	noCache bool,
	tags []string,
	buildArgs []string,
	labels []string,
	outputs []string,
	provenance string,
) (*Container, error) {
	args := []string{"/usr/bin/depot", "build", "."}

	if push {
		args = append(args, "--push")
	}
	if load {
		args = append(args, "--load")
	}
	if sbom {
		args = append(args, "--sbom=true")
	}
	if noCache {
		args = append(args, "--no-cache")
	}

	for _, tag := range tags {
		args = append(args, "--tag", tag)
	}

	for _, platform := range platforms {
		args = append(args, "--platform", string(platform))
	}

	for _, buildArg := range buildArgs {
		args = append(args, "--build-arg", string(buildArg))
	}

	for _, label := range labels {
		args = append(args, "--label", string(label))
	}

	for _, output := range outputs {
		args = append(args, "--output", string(output))
	}

	if dockerfile != "" {
		args = append(args, "--file", dockerfile)
	}

	if provenance != "" {
		args = append(args, "--provenance", provenance)
	}

	container := dag.Container().
		From("ghcr.io/depot/cli:2.46.0").
		WithMountedDirectory("/mnt", directory).
		WithEnvVariable("DEPOT_PROJECT_ID", project).
		WithSecretVariable("DEPOT_TOKEN", token).
		WithWorkdir("/mnt").
		WithEntrypoint(args)

	if dockerHost == "" {
		dockerHost = DefaultDockerHost
	}

	switch {
	case strings.HasPrefix(dockerHost, "unix://"):
		dockerHost = strings.TrimPrefix(dockerHost, "unix://")

		container = container.WithUnixSocket("/var/run/docker.sock", dag.Host().UnixSocket(dockerHost))
		container = container.WithEnvVariable("DOCKER_HOST", "unix:///var/run/docker.sock")
	case strings.HasPrefix(dockerHost, "tcp://"):
		container = container.WithEnvVariable("DOCKER_HOST", dockerHost)
	}

	return container, nil
}
