# Module: Depot

![dagger-min-version](https://img.shields.io/badge/dagger%20version-v0.9.3-yellow)

Daggerized version of [depot](https://depot.dev).

## Example

### build

```sh
dagger call -m github.com/depot/dagger-mod/depot \
  build --token $DEPOT_TOKEN --project $DEPOT_PROJECT --directory . --tags howdy/microservice:6.5.44  --load
```

### bake

```sh
dagger call -m github.com/depot/dagger-mod/depot \
  bake --token $DEPOT_TOKEN --project $DEPOT_PROJECT --directory . --bake-file docker-bake.hcl --load
```
