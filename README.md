# sitectl-wp

`sitectl-wp` simplifies the creation and operation of repositories created using the [LibOps WordPress template](https://github.com/libops/wp). It provides sitectl commands for WP-CLI, Composer, plugin and theme maintenance, database work, validation, and health checks.

Documentation: https://sitectl.libops.io/plugins/wordpress

## Requirements

- [`sitectl`](https://sitectl.libops.io/install).
- Docker with the Compose v2 plugin for local WordPress sites.
- No additional app-plugin dependency beyond core `sitectl`.

## Quick Start

Create a local WordPress site from the matching template:

```bash
sitectl create wp/default \
  --template-repo https://github.com/libops/wp \
  --path ./my-wordpress-site \
  --type local \
  --checkout-source template \
  --default-context
```

The template README is at https://github.com/libops/wp.

## Basic Operations

Use [`sitectl compose`](https://sitectl.libops.io/commands/compose) to start or inspect the stack:

```bash
sitectl compose up --remove-orphans -d
```

Use [`sitectl healthcheck`](https://sitectl.libops.io/commands/healthcheck) and [`sitectl validate`](https://sitectl.libops.io/commands/validate) to check the site:

```bash
sitectl healthcheck
sitectl validate
```

Use [`sitectl image`](https://sitectl.libops.io/commands/image) for local image or build-arg overrides:

```bash
sitectl image set --tag wp=nginx-1.30.3-php84
```

Use [`sitectl set`](https://sitectl.libops.io/commands/set) for component changes; it updates component-owned files immediately:

```bash
sitectl set ingress enabled --mode https-custom --domain wordpress.localhost
sitectl set ingress enabled --trusted-ip 203.0.113.10/32 --max-upload-size 2G --upload-timeout 10m
```

See the [WordPress plugin docs](https://sitectl.libops.io/plugins/wordpress) for WP-CLI, Composer, plugin/theme maintenance, lifecycle operations, and database helpers.

## License

`sitectl-wp` is licensed under the MIT License.
