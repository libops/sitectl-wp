# sitectl-wp

`sitectl-wp` simplifies the creation and operation of repositories created using the [LibOps WordPress template](https://github.com/libops/wp). It provides sitectl commands for WP-CLI, Composer, plugin and theme maintenance, database work, validation, and health checks.

Documentation: https://sitectl.libops.io/plugins/wordpress

## Requirements

- [`sitectl`](https://sitectl.libops.io/install) v1.7.0 or newer provides the RPC verifier SDK; promotion must pin the first core release that also includes `verify --strict` semantics.
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
sitectl verify --strict
```

## Behavioral verification

`sitectl verify --strict` compares runtime WordPress core with `composer.lock`, rejects a root database identity, checks the REST `wp/v2` namespace, validates WP-Cron events, verifies Bedrock home/site URLs and writable uploads, and confirms the managed administrator exists. Production verification is read-only.

Disposable CI may add a reversible media import/read/delete probe as the WordPress service account:

```bash
sitectl verify --strict --disposable
```

Never use `--disposable` for a retained customer site. The local verifier does not replace hosted acceptance for public DNS/TLS, external ingress, browser login, real mail delivery, externally triggered cron, or restore testing.

## Composer-owned updates

WordPress core, plugins, and themes are image content governed by `composer.json` and `composer.lock`. The named update helpers now update that checkout through Composer:

```bash
sitectl wp core update 7.0.2
sitectl wp plugin update akismet:5.7
sitectl wp theme update twentytwentyfour:^1.5
```

Every helper requires an explicit Composer constraint because the template deliberately pins exact package versions; silently selecting “latest” would weaken release reproducibility. Review and commit both Composer files, then rebuild and deploy. Raw `sitectl wp cli core update`, `plugin install|update|delete`, and `theme install|update|delete` fail with Composer guidance because those commands would mutate only the disposable running container. WP-CLI remains available for runtime state such as activation, users, options, cache, and database maintenance.

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
