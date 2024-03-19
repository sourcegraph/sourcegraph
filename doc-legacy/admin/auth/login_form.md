# Login form

The login form allows users to sign in to Sourcegraph using [configured auth providers](./index.md). 

<img alt="login form screenshot" src="https://sourcegraphstatic.com/docs/images/administration/auth/login_form.png" class="screenshot">

## Configuration

<span class="badge badge-note">Sourcegraph 5.1+</span>

These options do not apply to [`builtin`](./index.md#builtin-password-authentication) and 
[`http-header`](./index.md#http-authentication-proxies) auth providers.
- The builtin auth provider has its own login form.
- The HTTP header auth provider does not appear on the login form as it is applied on every request if configured.

### Change order of auth providers

When multiple auth providers are configured, the login form displays a login button for each of them. Default order 
of auth providers is hardcoded in the application.

The default order can be overriden with an optional `order` parameter. It is an integer and items 
will be sorted in natural order (1, 2, 3, ...). 

Example [site configuration](../config/site_config.md):
```json
{
  "auth.providers": {
    "builtin": {
      {
        "type": "builtin",
        "allowSignup": false
      },
    },
    {
      "type": "github",
      "order": 2
    },
    {
      "type": "gitlab",
      "order": 1
    }
  }
}
```

In this case, the GitLab auth provider will be shown above the GitHub auth provider on the login page.

<img alt="login form auth providers order screenshot" src="https://sourcegraphstatic.com/docs/images/administration/auth/login_form_order.png" class="screenshot">

Auth providers without `order` parameter will be put at the end of the auth providers list.

### Limit count of login options

By default, the login form shows up to 5 primary auth provider buttons on the page. Other auth providers can be reached 
with the `Other login methods` button.

This default can be changed, e.g. in case there are 1 or 2 preferred methods for users to login with. 
For example there might be a different auth provider setup for regular engineers and a different one for site admins. 
It makes sense to only show the default one to engineers to reduce confusion of regular users. 

There is a [site configuration](../config/site_config.md) parameter `auth.primaryLoginProvidersCount`:
```json
{
  "auth.primaryLoginProvidersCount": 1,
  // ...
}
```

In the example above, there will be only 1 primary provider, all the other providers will be shown on the next screen, 
when the `Other login methods` button is clicked.

<img alt="login form limit auth providers" src="https://sourcegraphstatic.com/docs/images/administration/auth/login_form_limit.png" class="screenshot">

### Change label of auth provider button

By default Sourcegraph shows a button for each auth provider, such as `Continue with GitHub`. The text label for the button 
is created from 2 parts: `Continue with` prefix and `Github`. These can be controlled with `displayPrefix` and `displayName` 
optional parameters of auth provider in [site configuration](../config/site_config.md):

Example [site configuration](../config/site_config.md):
```json
{
  "auth.providers": [
    {
      "type": "github",
      "displayName": "GitHub Enterprise",
      "displayPrefix": "Login with"
    },
    {
      "type": "gitlab",
      "displayName": "GitLab",
      "displayPrefix": "Login with"
    }
  ]
}
```

The example configuration above will render 2 buttons, `Login with GitHub Enterprise` and `Login with GitLab`.

<img alt="login form button label" src="https://sourcegraphstatic.com/docs/images/administration/auth/login_form_label.png" class="screenshot">

By default, the `displayPrefix` will be `Continue with` and `displayName` will be infered from the auth provider type.

### Hide auth provider

> NOTE: Hiding an auth provider is mostly useful for development purposes and special cases.

It is also possible to hide the auth provider from the login form completely. Auth providers have a `hidden` boolean property. 
See the [site configuration](../config/site_config.md) example below:
```json
{
  "auth.providers": [
    {
      "type": "github",
      "hidden": true,
      // ...
    },
    {
      "type": "gitlab",
      // ...
    }
  ]
}
```

In this example, the GitHub auth provider will not be shown on the login form at all. Only the GitLab auth provider will be shown. 
