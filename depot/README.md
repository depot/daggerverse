# Module: Depot

![dagger-min-version](https://img.shields.io/badge/dagger%20version-v0.9.3-yellow)

Daggerized version of [depot](https://depot.dev).

## CLI Examples

### Build and run container

```sh
dagger call -m github.com/depot/daggerverse/depot \
  build --token $DEPOT_TOKEN --project $DEPOT_PROJECT --directory . container
```

### Build image and print image size in bytes

```sh
dagger call -m github.com/depot/daggerverse/depot \
  build --token $DEPOT_TOKEN --project $DEPOT_PROJECT --directory . image-bytes
```

### Build image and print software bill of materials (SBOM)

```sh
dagger call -m github.com/depot/daggerverse/depot \
  build --token $DEPOT_TOKEN --project $DEPOT_PROJECT --directory . --sbom sbom
```

### Run bake to build many containers.

```sh
dagger call -m github.com/depot/daggerverse/depot \
  bake --token $DEPOT_TOKEN --project $DEPOT_PROJECT --directory . --bake-file docker-bake.hcl
```

## API Examples

```go

```
