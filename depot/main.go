package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"runtime"
	"strings"
)

const (
	DefaultDockerHost = "unix:///var/run/docker.sock"
)

type Depot struct{}

// example usage: "dagger call build --token $DEPOT_TOKEN --project $DEPOT_PROJECT --directory .  --tags howdy/microservice:6.5.44  --load"
func (m *Depot) Build(ctx context.Context,
	// depot CLI version (default: latest)
	depotVersion Optional[string],
	// depot token
	token *Secret,
	// depot project id
	project string,
	// source context directory for build
	directory *Directory,
	// path to dockerfile (default: Dockerfile)
	dockerfile Optional[string],
	// target platforms for build
	platforms Optional[[]Platform],
	// docker host (default: unix:///var/run/docker.sock)
	dockerHost Optional[string],
	// load image into local docker daemon.
	load Optional[bool],
	// produce software bill of materials for image
	sbom Optional[bool],
	// do not use layer cache when building the image
	noCache Optional[bool],
	// lint dockerfile
	lint Optional[bool],
	// name and tag for output image
	tags Optional[[]string],
	buildArgs Optional[[]string],
	labels Optional[[]string],
	outputs Optional[[]string],
	provenance Optional[string],
) (*Container, error) {
	return build(
		ctx,
		depotVersion.GetOr(""),
		token,
		project,
		directory,
		dockerfile.GetOr(""),
		platforms.GetOr([]Platform{}),
		dockerHost.GetOr(""),
		load.GetOr(false),
		sbom.GetOr(false),
		noCache.GetOr(false),
		lint.GetOr(false),
		tags.GetOr([]string{}),
		buildArgs.GetOr([]string{}),
		labels.GetOr([]string{}),
		outputs.GetOr([]string{}),
		provenance.GetOr(""),
	)
}

// example usage: "dagger call bake --token $DEPOT_TOKEN --project $DEPOT_PROJECT --directory . --bake-file docker-bake.hcl --load"
func (m *Depot) Bake(ctx context.Context,
	// depot CLI version (default: latest)
	depotVersion Optional[string],
	// depot token
	token *Secret,
	// depot project id
	project string,
	// source context directory for build
	directory *Directory,
	// path to bake definition file
	bakeFile string,
	// docker host (default: unix:///var/run/docker.sock)
	dockerHost Optional[string],
	// load image into local docker daemon.
	load Optional[bool],
	// produce software bill of materials for image
	sbom Optional[bool],
	// do not use layer cache when building the image
	noCache Optional[bool],
	provenance Optional[string],
	// lint dockerfile
	lint Optional[bool],
) (*Container, error) {
	return bake(
		ctx,
		depotVersion.GetOr(""),
		token,
		project,
		directory,
		bakeFile,
		dockerHost.GetOr(""),
		load.GetOr(false),
		sbom.GetOr(false),
		noCache.GetOr(false),
		lint.GetOr(false),
		provenance.GetOr(""),
	)
}

// example usage: "dagger call builder with-token --token $DEPOT_TOKEN  with-project  --project $DEPOT_RPOJECT with-directory --directory . with-tag --tag howdy/microservice:6.5.44  with-load run"
func (m *Depot) Builder() *Builder {
	return &Builder{}
}

type Builder struct {
	DepotVersion string

	// DockerHost is used for --load.
	DockerHost string

	Token      *Secret
	Project    string
	Directory  *Directory
	Dockerfile string

	Platforms []Platform

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

func (m *Builder) WithDepotVersion(version string) *Builder {
	m.DepotVersion = version
	return m
}

func (m *Builder) WithToken(token *Secret) *Builder {
	m.Token = token
	return m
}

func (m *Builder) WithProject(project string) *Builder {
	m.Project = project
	return m
}

func (m *Builder) WithDirectory(directory *Directory) *Builder {
	m.Directory = directory
	return m
}

func (m *Builder) WithDockerfile(dockerfile string) *Builder {
	m.Dockerfile = dockerfile
	return m
}

func (m *Builder) WithNoCache() *Builder {
	m.NoCache = true
	return m
}

func (m *Builder) WithLoad() *Builder {
	m.Load = true
	return m
}

func (m *Builder) WithSBOM() *Builder {
	m.SBOM = true
	return m
}

func (m *Builder) WithLint() *Builder {
	m.Lint = true
	return m
}

func (m *Builder) WithPlatform(platform Platform) *Builder {
	m.Platforms = append(m.Platforms, platform)
	return m
}

func (m *Builder) WithTag(tag string) *Builder {
	m.Tags = append(m.Tags, tag)
	return m
}

func (m *Builder) WithBuildArg(arg string) *Builder {
	m.BuildArgs = append(m.BuildArgs, arg)
	return m
}

func (m *Builder) WithLabel(label string) *Builder {
	m.Labels = append(m.Labels, label)
	return m
}

func (m *Builder) WithOutput(output string) *Builder {
	m.Outputs = append(m.Outputs, output)
	return m
}

func (m *Builder) WithProvenance(provenance string) *Builder {
	m.Provenance = provenance
	return m
}

func (m *Builder) WithDockerHost(dockerHost string) *Builder {
	m.DockerHost = dockerHost
	return m
}

func (m *Builder) Run(ctx context.Context) (*Container, error) {
	return build(
		ctx,
		m.DepotVersion,
		m.Token,
		m.Project,
		m.Directory,
		m.Dockerfile,
		m.Platforms,
		m.DockerHost,
		m.Load,
		m.SBOM,
		m.NoCache,
		m.Lint,
		m.Tags,
		m.BuildArgs,
		m.Labels,
		m.Outputs,
		m.Provenance,
	)
}

func build(ctx context.Context,
	depotVersion string,
	token *Secret,
	project string,
	directory *Directory,
	dockerfile string,
	platforms []Platform,
	dockerHost string,
	load bool,
	sbom bool,
	noCache bool,
	lint bool,
	tags []string,
	buildArgs []string,
	labels []string,
	outputs []string,
	provenance string,
) (*Container, error) {
	args := []string{"/usr/bin/depot", "build", "."}

	if load {
		args = append(args, "--load")
	}
	if sbom {
		args = append(args, "--sbom=true")
	}
	if noCache {
		args = append(args, "--no-cache")
	}
	if lint {
		args = append(args, "--lint")
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

	if depotVersion == "" {
		var err error
		depotVersion, err = latestDepotVersion()
		if err != nil {
			return nil, err
		}
	}

	depotImage := fmt.Sprintf("ghcr.io/depot/cli:%s", depotVersion)

	container := dag.Container().
		From(depotImage).
		WithMountedDirectory("/mnt", directory).
		WithEnvVariable("DEPOT_PROJECT_ID", project).
		WithSecretVariable("DEPOT_TOKEN", token).
		WithWorkdir("/mnt")

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
	// WithExec must come after WithUnixSocket and WithEnvVariable please.
	return container.WithExec(args, ContainerWithExecOpts{SkipEntrypoint: true}), nil
}

func bake(ctx context.Context,
	depotVersion string,
	token *Secret,
	project string,
	directory *Directory,
	bakeFile string,
	dockerHost string,
	load bool,
	sbom bool,
	noCache bool,
	lint bool,
	provenance string,
) (*Container, error) {
	args := []string{"/usr/bin/depot", "bake", "-f", bakeFile}

	if load {
		args = append(args, "--load")
	}
	if sbom {
		args = append(args, "--sbom=true")
	}
	if noCache {
		args = append(args, "--no-cache")
	}
	if lint {
		args = append(args, "--lint")
	}

	if provenance != "" {
		args = append(args, "--provenance", provenance)
	}

	if depotVersion == "" {
		var err error
		depotVersion, err = latestDepotVersion()
		if err != nil {
			return nil, err
		}
	}

	depotImage := fmt.Sprintf("ghcr.io/depot/cli:%s", depotVersion)

	container := dag.Container().
		From(depotImage).
		WithMountedDirectory("/mnt", directory).
		WithEnvVariable("DEPOT_PROJECT_ID", project).
		WithSecretVariable("DEPOT_TOKEN", token).
		WithWorkdir("/mnt")

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
	// WithExec must come after WithUnixSocket and WithEnvVariable please.
	return container.WithExec(args, ContainerWithExecOpts{SkipEntrypoint: true}), nil
}

func latestDepotVersion() (string, error) {
	url := fmt.Sprintf("https://dl.depot.dev/cli/release/%s/%s/latest", runtime.GOOS, runtime.GOARCH)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", err
	}

	req.Header.Add("Content-Type", "application/json")
	//req.Header.Add("User-Agent", Agent())

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	version := struct {
		Version string `json:"version"`
	}{}
	err = json.NewDecoder(resp.Body).Decode(&version)
	if err != nil {
		return "", err
	}

	return version.Version, nil
}
