# sitectl-wp

`sitectl-wp` adds WordPress create metadata, WP-CLI helpers, Composer commands, plugin and theme maintenance, lifecycle operations, database helpers, validation, and health checks to [`sitectl`](https://sitectl.libops.io). It works with the [LibOps WordPress template](https://github.com/libops/wp).

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

Use [`sitectl set`](https://sitectl.libops.io/commands/set) and [`sitectl converge`](https://sitectl.libops.io/commands/converge) for component changes:

```bash
sitectl set upload-limits enabled --max-upload-size 2G --upload-timeout 10m
sitectl converge
```

See the [WordPress plugin docs](https://sitectl.libops.io/plugins/wordpress) for WP-CLI, Composer, plugin/theme maintenance, lifecycle operations, and database helpers.

## License

`sitectl-wp` is licensed under the MIT License.
