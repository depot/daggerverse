// Depot is a remote container build service with native Intel & Arm support and persistent layer caching for blazing fast builds.
//
// The Depot module can be used to route any container image build to our remote build service. You can use it to build container images on
// fast native Intel & Arm CPUs with persistent layer cache on NVMe disks. We have functions for both depot build and depot bake. 
//
// With build, we build your container image for the Dockerfile you specify and return you the built container to use in your pipeline.
// With bake, you can pass in your bake file and we will build all of the targets in your bake file concurrently.
// 
// For more examples of cool things you can do with Dagger + Depot, check out our README in our Daggerverse repo.

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"runtime"
	"strings"
)

type Depot struct{}

type BuildArtifact struct {
	// depot token
	Token *Secret
	// depot project id
	Project string

	Target string

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

// Build builds a container image artifact from a Dockerfile using https://depot.dev.
//
// Example usage: `dagger call build --token env:DEPOT_TOKEN --project $DEPOT_PROJECT --directory . --lint container`
func (m *Depot) Build(ctx context.Context,
	// Depot CLI version
	// +optional
	depotVersion string,
	// Depot token
	token *Secret,
	// Depot project id
	project string,
	// Source context directory for build
	directory *Directory,
	// Path to dockerfile
	// +optional
	// +default="Dockerfile"
	dockerfile string,
	// Platforms are architecture and OS combinations for which to build the image.
	// +optional
	// +default=null
	platforms []Platform,
	// Produce software bill of materials for image
	// +optional
	// +default=false
	sbom bool,
	// D not use layer cache when building the image
	// +optional
	// +default=false
	noCache bool,
	// Do not save the image to the depot ephemeral registry
	// +optional
	// +default=false
	noSave bool,
	// Lint dockerfile
	// +optional
	// +default=false
	lint bool,
	// +optional
	// +default=null
	buildArgs []string,
	// Labels to apply to the image
	// +optional
	// +default=null
	labels []string,
	// Outputs override the default
	// +optional
	// +default=null
	outputs []string,
	// +optional
	provenance string,
) (*BuildArtifact, error) {
	args := []string{"/usr/bin/depot", "build", ".", "--metadata-file=metadata.json"}
	// Always save unless one specifies --no-save.
	if !noSave {
		args = append(args, "--save")
	}

	for _, platform := range platforms {
		args = append(args, "--platform", string(platform))
	}

	for _, buildArg := range buildArgs {
		args = append(args, "--build-arg", buildArg)
	}

	for _, label := range labels {
		args = append(args, "--label", label)
	}

	for _, output := range outputs {
		args = append(args, "--output", output)
	}

	if dockerfile != "" {
		args = append(args, "--file", dockerfile)
	}

	if provenance != "" {
		args = append(args, "--provenance", provenance)
	}
	if sbom {
		// produce and download sboms
		args = append(args, "--sbom=true", "--sbom-dir=/mnt/sboms")
	}
	if noCache {
		args = append(args, "--no-cache")
	}
	if lint {
		args = append(args, "--lint")
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
		WithEnvVariable("DEPOT_DISABLE_OTEL", "true").
		WithSecretVariable("DEPOT_TOKEN", token).
		WithWorkdir("/mnt")

	exec := container.WithExec(args, ContainerWithExecOpts{SkipEntrypoint: true})
	metadataFile := exec.File("metadata.json")
	buf, err := metadataFile.Contents(ctx)
	if err != nil {
		return nil, err
	}

	metadata := BuildMetadata{}
	err = json.Unmarshal([]byte(buf), &metadata)
	if err != nil {
		return nil, err
	}

	artifact := &BuildArtifact{
		Token:     token,
		Project:   project,
		ImageName: metadata.ImageName,
		Size:      metadata.Size(),
	}

	if sbom {
		artifact.SBOMDir = exec.Directory("/mnt/sboms")
	}

	return artifact, nil
}

// Bake builds many containers using https://depot.dev.
//
// example usage: `dagger call bake --token $DEPOT_TOKEN --project $DEPOT_PROJECT --directory . --bake-file docker-bake.hcl`
func (m *Depot) Bake(ctx context.Context,
	// depot CLI version
	// +optional
	depotVersion string,
	// depot token
	token *Secret,
	// depot project id
	project string,
	// source context directory for build
	directory *Directory,
	// path to bake definition file
	bakeFile string,
	// produce software bill of materials for image
	// +optional
	// +default=false
	sbom bool,
	// do not use layer cache when building the image
	// +optional
	// +default=false
	noCache bool,
	// Do not save the image to the depot ephemeral registry
	// +optional
	// +default=false
	noSave bool,
	// lint dockerfile
	// +optional
	// +default=false
	lint bool,
	// +optional
	provenance string,
) (*Artifacts, error) {
	args := []string{"/usr/bin/depot", "bake", "-f", bakeFile, "--metadata-file=metadata.json"}
	// Always save unless one specifies --no-save.
	if !noSave {
		args = append(args, "--save")
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

	depotImage := fmt.Sprintf("public.ecr.aws/depot/cli:%s", depotVersion)

	container := dag.Container().
		From(depotImage).
		WithMountedDirectory("/mnt", directory).
		WithEnvVariable("DEPOT_PROJECT_ID", project).
		WithEnvVariable("DEPOT_DISABLE_OTEL", "true").
		WithSecretVariable("DEPOT_TOKEN", token).
		WithWorkdir("/mnt")

	// WithExec must come after WithUnixSocket and WithEnvVariable please.
	exec := container.WithExec(args, ContainerWithExecOpts{SkipEntrypoint: true})
	metadataFile := exec.File("metadata.json")
	buf, err := metadataFile.Contents(ctx)
	if err != nil {
		return nil, err
	}
	var bakeMetadata BakeMetadata
	err = json.Unmarshal([]byte(buf), &bakeMetadata)
	if err != nil {
		return nil, err
	}

	artifacts := make([]*BuildArtifact, 0, len(bakeMetadata.Targets))
	for target, metadata := range bakeMetadata.Targets {
		imageName := fmt.Sprintf("registry.depot.dev/%s:%s-%s", bakeMetadata.DepotBuild.ProjectID, bakeMetadata.DepotBuild.BuildID, target)
		artifact := &BuildArtifact{
			Token:     token,
			Project:   project,
			Target:    target,
			ImageName: imageName,
			Size:      metadata.Size(),
			// TODO: sboms
		}
		artifacts = append(artifacts, artifact)
	}

	return &Artifacts{Artifacts: artifacts}, nil
}

type Artifacts struct {
	Artifacts []*BuildArtifact
}

func (a *Artifacts) Get(target string) (*BuildArtifact, error) {
	for _, artifact := range a.Artifacts {
		if artifact.Target == target {
			return artifact, nil
		}
	}

	targets := make([]string, 0, len(a.Artifacts))
	for _, artifact := range a.Artifacts {
		targets = append(targets, artifact.Target)
	}

	return nil, fmt.Errorf("no such artifact target %s. valid targets: %s", target, strings.Join(targets, ", "))
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

type BuildMetadata struct {
	DepotBuild DepotBuild `json:"depot.build,omitempty"`
	Metadata
}

type BakeMetadata struct {
	DepotBuild DepotBuild
	Targets    map[string]Metadata
}

func (m *BakeMetadata) UnmarshalJSON(d []byte) error {
	var obj map[string]json.RawMessage
	err := json.Unmarshal(d, &obj)
	if err != nil {
		return err
	}

	m.Targets = make(map[string]Metadata)
	for k, v := range obj {
		if k == "depot.build" {
			err = json.Unmarshal(obj["depot.build"], &m.DepotBuild)
			if err != nil {
				return err
			}
		} else {
			var md Metadata
			err = json.Unmarshal(v, &md)
			if err != nil {
				return err
			}
			m.Targets[k] = md
		}
	}
	return nil
}

type DepotBuild struct {
	BuildID   string   `json:"buildID,omitempty"`
	ProjectID string   `json:"projectID,omitempty"`
	Targets   []string `json:"targets,omitempty"`
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
