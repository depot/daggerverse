# Module: Depot

![dagger-min-version](https://img.shields.io/badge/dagger%20version-v0.9.3-yellow)

Daggerized version of [depot](https://depot.dev).

## Example

```sh
dagger -m github.com/depot/dagger-mod/depot call \
  depot build --token $DEPOT_TOKEN --project $DEPOT_PROJECT --directory . --tags howdy/microservice:6.5.44  --load
```

