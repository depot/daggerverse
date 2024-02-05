package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"runtime"
)

type Depot struct {
	// depot CLI version (default: latest)
	DepotVersion string
	// Depot token
	Token *Secret
	// Depot project id
	Project string
	// Source context directory for build
	Directory *Directory
	// Path to dockerfile (default: Dockerfile)
	Dockerfile string
	// target platforms for build
	Platforms []Platform

	// produce software bill of materials for image
	SBOM bool
	// lint dockerfile
	Lint bool
	// do not use layer cache when building the image
	NoCache bool
	// do not save the image to the depot ephemeral registry
	NoSave bool

	BuildArgs []string
	Labels    []string
	Outputs   []string

	Provenance string
}

type BuildArtifact struct {
	// depot token
	Token *Secret
	// depot project id
	Project string

	SBOMDir   *Directory
	ImageName string
	Size      int64
}

// Creates a container from the recently built image artifact.
func (b *BuildArtifact) Container() *Container {
	return dag.Container().WithRegistryAuth("registry.depot.dev", "x-token", b.Token).From(b.ImageName)
}

// Returns the size in bytes of the image.
func (b *BuildArtifact) ImageBytes() int64 {
	// This is the sum of the size of the image config and all layers.
	// Note that this is the compressed layer size.  Images are stored compressed in the registry.
	// The on-disk, uncompressed size is not available.
	return b.Size
}

// Returns an SBOM if built option `--sbom` was requested.
// Returns an error if the build did not produce SBOMs.
func (b *BuildArtifact) SBOM(ctx context.Context) (*File, error) {
	if b.SBOMDir == nil {
		return nil, fmt.Errorf("sbom not generated; use --sbom")
	}

	paths, err := b.SBOMDir.Entries(ctx)
	if err != nil {
		return nil, err
	}

	var sboms []*File
	for _, path := range paths {
		sbomFile := b.SBOMDir.File(path)
		sboms = append(sboms, sbomFile)
	}
	if len(sboms) == 0 {
		return nil, fmt.Errorf("no sboms found")
	}

	return sboms[0], nil
}

// example usage: `dagger call build --token $DEPOT_TOKEN --project $DEPOT_PROJECT --directory . --lint container`
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
	// produce software bill of materials for image
	sbom Optional[bool],
	// do not use layer cache when building the image
	noCache Optional[bool],
	// do not save the image to the depot ephemeral registry
	noSave Optional[bool],
	// lint dockerfile
	lint Optional[bool],
	buildArgs Optional[[]string],
	labels Optional[[]string],
	outputs Optional[[]string],
	provenance Optional[string],
) (*BuildArtifact, error) {
	d := &Depot{
		DepotVersion: depotVersion.GetOr(""),
		Token:        token,
		Project:      project,
		Directory:    directory,
		Dockerfile:   dockerfile.GetOr(""),
		Platforms:    platforms.GetOr([]Platform{}),
		SBOM:         sbom.GetOr(false),
		NoCache:      noCache.GetOr(false),
		NoSave:       noSave.GetOr(false),
		Lint:         lint.GetOr(false),
		BuildArgs:    buildArgs.GetOr([]string{}),
		Labels:       labels.GetOr([]string{}),
		Outputs:      outputs.GetOr([]string{}),
		Provenance:   provenance.GetOr(""),
	}
	return build(ctx, d)
}

// example usage: `dagger call bake --token $DEPOT_TOKEN --project $DEPOT_PROJECT --directory . --bake-file docker-bake.hcl`
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
		sbom.GetOr(false),
		noCache.GetOr(false),
		lint.GetOr(false),
		provenance.GetOr(""),
	)
}

func build(ctx context.Context, d *Depot) (*BuildArtifact, error) {
	args := []string{"/usr/bin/depot", "build", ".", "--metadata-file=metadata.json"}
	// Always save unless one specifies --no-save.
	if !d.NoSave {
		args = append(args, "--save")
	}

	for _, platform := range d.Platforms {
		args = append(args, "--platform", string(platform))
	}

	for _, buildArg := range d.BuildArgs {
		args = append(args, "--build-arg", buildArg)
	}

	for _, label := range d.Labels {
		args = append(args, "--label", label)
	}

	for _, output := range d.Outputs {
		args = append(args, "--output", output)
	}

	if d.Dockerfile != "" {
		args = append(args, "--file", d.Dockerfile)
	}

	if d.Provenance != "" {
		args = append(args, "--provenance", d.Provenance)
	}
	if d.SBOM {
		// produce and download sboms
		args = append(args, "--sbom=true", "--sbom-dir=/mnt/sboms")
	}
	if d.NoCache {
		args = append(args, "--no-cache")
	}
	if d.Lint {
		args = append(args, "--lint")
	}
	if d.DepotVersion == "" {
		var err error
		d.DepotVersion, err = latestDepotVersion()
		if err != nil {
			return nil, err
		}
	}

	depotImage := fmt.Sprintf("public.ecr.aws/depot/cli:%s", d.DepotVersion)

	container := dag.Container().
		From(depotImage).
		WithMountedDirectory("/mnt", d.Directory).
		WithEnvVariable("DEPOT_PROJECT_ID", d.Project).
		WithEnvVariable("DEPOT_DISABLE_OTEL", "true").
		WithSecretVariable("DEPOT_TOKEN", d.Token).
		WithWorkdir("/mnt")

	exec := container.WithExec(args, ContainerWithExecOpts{SkipEntrypoint: true})
	metadataFile := exec.File("metadata.json")
	buf, err := metadataFile.Contents(ctx)
	if err != nil {
		return nil, err
	}

	metadata := Metadata{}
	err = json.Unmarshal([]byte(buf), &metadata)
	if err != nil {
		return nil, err
	}

	artifact := &BuildArtifact{
		Token:     d.Token,
		Project:   d.Project,
		ImageName: metadata.ImageName,
		Size:      metadata.Size(),
	}

	if d.SBOM {
		artifact.SBOMDir = exec.Directory("/mnt/sboms")
	}

	return artifact, nil
}

func bake(ctx context.Context,
	depotVersion string,
	token *Secret,
	project string,
	directory *Directory,
	bakeFile string,
	sbom bool,
	noCache bool,
	lint bool,
	provenance string,
) (*Container, error) {
	args := []string{"/usr/bin/depot", "bake", "-f", bakeFile}

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

	depotImage := fmt.Sprintf("public.ecr.aws/depot/cli:%s", depotVersion)

	container := dag.Container().
		From(depotImage).
		WithMountedDirectory("/mnt", directory).
		WithEnvVariable("DEPOT_PROJECT_ID", project).
		WithSecretVariable("DEPOT_TOKEN", token).
		WithWorkdir("/mnt")

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

type Metadata struct {
	// This is the index of the image in the manifest list.
	ContainerImageDescriptor OCIDescriptor `json:"containerimage.descriptor,omitempty"`
	DepotBuild               DepotBuild    `json:"depot.build,omitempty"`
	// Use this for the image name.
	ImageName string `json:"image.name,omitempty"`
	// Ignore Image configs for now.
	Manifests []Manifest `json:"manifests,omitempty"`

	// The metadata format is a bit of an odd duck.  If it is a multi-platform build, it will have
	// a containerimage.buildinfo/PLATFORM section.  If it is a single platform build, it will have a
	// containerimage.buildinfo section but no way to know the platform.
	//ContainerimageBuildinfo           *struct{} `json:"containerimage.buildinfo,omitempty"`
	//ContainerimageBuildinfoLinuxArm64 *struct{} `json:"containerimage.buildinfo/linux/arm64,omitempty"`
	//ContainerimageBuildinfoLinuxAmd64 *struct{} `json:"containerimage.buildinfo/linux/amd64,omitempty"`
}

func (m *Metadata) Size() int64 {
	size := m.ContainerImageDescriptor.Size
	for _, manifest := range m.Manifests {
		size += manifest.Config.Size
		for _, layer := range manifest.Layers {
			size += layer.Size
		}
	}
	return size
}

type DepotBuild struct {
	BuildID   string `json:"buildID,omitempty"`
	ProjectID string `json:"projectID,omitempty"`
}

type Manifest struct {
	SchemaVersion int             `json:"schemaVersion,omitempty"`
	MediaType     string          `json:"mediaType,omitempty"`
	Config        OCIDescriptor   `json:"config,omitempty"`
	Layers        []OCIDescriptor `json:"layers,omitempty"`
}

type OCIDescriptor struct {
	MediaType string `json:"mediaType,omitempty"`
	Digest    string `json:"digest,omitempty"`
	Size      int64  `json:"size,omitempty"`
}
