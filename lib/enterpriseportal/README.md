# Enterprise Portal services

`lib/enterpriseportal` defines the gRPC services implemented by Enterprise Portal. Core functionality are defined in `subscriptions/v1`, with extensions defined as separate services implemented by Enterprise Portal, such as `codyaccess/v1`.

All RPCs follow the API design guidelines at https://google.aip.dev/, exceptions are otherwise noted.

To regenerate all relevant bindings:

```sh
sg gen buf \
  lib/enterpriseportal/subscriptions/v1/buf.gen.yaml \
  lib/enterpriseportal/codyaccess/v1/buf.gen.yaml \
  lib/enterpriseportal/subscriptionlicensechecks/v1/buf.gen.yaml
```

> [!CAUTION]
> These APIs have **production dependents**. Make changes with extreme care for backwards-compatibility.
