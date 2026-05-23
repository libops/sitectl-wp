# sitectl-wp

`sitectl-wp` is the LibOps sitectl plugin for WordPress.

It registers a first-class create definition for `https://github.com/libops/wp` so the stack can be installed with:

```bash
sitectl create wp
```

It also provides context-aware helpers:

- `sitectl wp build`
- `sitectl wp init`
- `sitectl wp up`
- `sitectl wp down`
- `sitectl wp status`
- `sitectl wp logs [SERVICE...]`
- `sitectl wp rollout`

WordPress-specific helpers:

- `sitectl wp cli [WP-CLI args...]`
- `sitectl wp composer [Composer args...]`
- `sitectl wp plugin list|status|update`
- `sitectl wp theme list|status|update`
- `sitectl wp core update-db`
- `sitectl wp cache flush`
- `sitectl wp db update|export|import`
