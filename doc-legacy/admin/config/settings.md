# Settings

Settings provide the ability to customize and control the Sourcegraph UI and user-specific features. They do not configure operational aspects of the instance (which are set in the [site configuration](site_config.md)).

Settings can be set at the global level (by site admins), the organization level (by organization members), and at the individual user level.

<div class="text-center">
  <object data="settings-cascade.svg" type="image/svg+xml" style="width:80%;"></object>
</div>

## Editing global settings (for site admins)

Global settings are found in **Site admin > Global settings** while links to organization and user settings are found in the user dropdown menu.

After setting or changing certain values in **Site admin > Global settings** the frontend will restart automatically or
you might be asked to restart the frontend for the changes to take effect.
In case of a Kubernetes deployment this can be done as follows:

```
bash
kubectl delete pods -l app=sourcegraph-frontend
```

## Reference

Settings options and their default values are shown below.

<div markdown-func=jsonschemadoc jsonschemadoc:path="admin/config/settings.schema.json">[View page on docs.sourcegraph.com](https://docs.sourcegraph.com/doc/admin/config/settings) to see rendered content.</div>

## Additional details on settings

### Notices

Notices can be added in global, organization, or user settings. The `notices` setting is a list of configuration consisting of three elements:

1. `message`: the markdown copy to be displayed in the banner
1. `location`: where the banner will be shown. Either on the home page with `"home"` or at the top of the page with `"top"`
1. `dismissible`: boolean (`true` or `false`). If true, users will be able to close the notice and not see it again. If false, it will persist on the instance until the configuration is removed.

### Example settings:

```json
"notices": [
  {
    "message": "Your message here! [Include a link for more information](http://example.com).",
    "location": "top",
    "dismissible": true
  }
]
```
