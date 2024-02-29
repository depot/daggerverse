# Module: Depot

![dagger-min-version](https://img.shields.io/badge/dagger%20version-v0.10.0-yellow)

Daggerized version of [depot](https://depot.dev).

## CLI Examples

### Build and run container

```sh
dagger call -m github.com/depot/daggerverse/depot \
  build --token env:DEPOT_TOKEN --project $DEPOT_PROJECT --directory . container
```

### Build image and print image size in bytes

```sh
dagger call -m github.com/depot/daggerverse/depot \
  build --token env:DEPOT_TOKEN --project $DEPOT_PROJECT --directory . image-bytes
```

### Build image and print software bill of materials (SBOM)

```sh
dagger call -m github.com/depot/daggerverse/depot \
  build --token env:DEPOT_TOKEN --project $DEPOT_PROJECT --directory . --sbom sbom
```

### Run bake to build many containers.

```sh
dagger call -m github.com/depot/daggerverse/depot \
  bake --token env:DEPOT_TOKEN --project $DEPOT_PROJECT --directory . --bake-file docker-bake.hcl
```

## API Examples

### Go

First, install the module.

```sh
dagger mod install github.com/depot/daggerverse/depot
```

This example builds an image and publishes if size is less than 100MB.

```go
// example usage: `dagger call publish-image-if-small --directory . --depot-token env:DEPOT_TOKEN --project $DEPOT_PROJECT_ID ----max-bytes 1000000 --image-address ghcr.io/my-project/my-image:latest`
func (m *MyModule) PublishImageIfSmall(ctx context.Context, depotToken *Secret, project string, directory *Directory, maxBytes int, imageAddress string) (string, error) {
	artifact := dag.Depot().Build(depotToken, project, directory)
	bytes, err := artifact.ImageBytes(ctx)
	if err != nil {
		return "", err
	}
	if bytes > maxBytes {
		return "", fmt.Errorf("image is too large: %d bytes", bytes)
	}

	return artifact.Container().Publish(ctx, imageAddress)
}
```

Here is an example that builds an image, gets image's SBOM and uses
Anchore's [grype](https://github.com/anchore/grype) to fail if any
high-severity CVEs are found.

```go
// example usage `dagger call check-cves --depot-token env:DEPOT_TOKEN --project $DEPOT_PROJECT_ID --directory .`
func (m *MyModule) CheckCVEs(ctx context.Context, depotToken *Secret, project string, directory *Directory) (string, error) {
	artifact := dag.Depot().Build(depotToken, project, directory, DepotBuildOpts{Sbom: true})
	sbomFile := artifact.Sbom()
	return dag.
		Container().
		From("anchore/grype:latest").
		WithFile("/mnt/sbom.spdx.json", sbomFile).
		WithExec([]string{"sbom:/mnt/sbom.spdx.json", "--fail-on=high"}).
		Stdout(ctx)
}
```
