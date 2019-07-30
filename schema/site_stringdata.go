// Code generated by stringdata. DO NOT EDIT.

package schema

// SiteSchemaJSON is the content of the file "site.schema.json".
const SiteSchemaJSON = `{
  "$schema": "http://json-schema.org/draft-07/schema#",
  "$id": "site.schema.json#",
  "title": "Site configuration",
  "description": "Configuration for a Sourcegraph site.",
  "allowComments": true,
  "type": "object",
  "additionalProperties": false,
  "properties": {
    "dontIncludeSymbolResultsByDefault": {
      "description": "Set to ` + "`" + `true` + "`" + ` to not include symbol results if no ` + "`" + `type:` + "`" + ` filter was given",
      "type": "boolean",
      "group": "Search"
    },
    "disableBuiltInSearches": {
      "description": "Whether built-in searches should be hidden on the Searches page.",
      "type": "boolean",
      "group": "Search"
    },
    "search.index.enabled": {
      "description": "Whether indexed search is enabled. If unset Sourcegraph detects the environment to decide if indexed search is enabled. Indexed search is RAM heavy, and is disabled by default in the single docker image. All other environments will have it enabled by default. The size of all your repository working copies is the amount of additional RAM required.",
      "type": "boolean",
      "!go": { "pointer": true },
      "group": "Search"
    },
    "search.largeFiles": {
      "description": "A list of file glob patterns where matching files will be indexed and searched regardless of their size. The glob pattern syntax can be found here: https://golang.org/pkg/path/filepath/#Match.",
      "type": "array",
      "items": {
        "type": "string"
      },
      "group": "Search",
      "examples": [["go.sum", "package-lock.json", "*.thrift"]]
    },
    "experimentalFeatures": {
      "description": "Experimental features to enable or disable. Features that are now enabled by default are marked as deprecated.",
      "type": "object",
      "additionalProperties": false,
      "properties": {
        "discussions": {
          "description": "Enables the code discussions experiment.",
          "type": "string",
          "enum": ["enabled", "disabled"],
          "default": "disabled"
        },
        "statusIndicator": {
          "description": "Enables the external service status indicator in the navigation bar.",
          "type": "string",
          "enum": ["enabled", "disabled"],
          "default": "enabled"
        }
      },
      "group": "Experimental",
      "hide": true
    },
    "corsOrigin": {
      "description": "Only required when using the Phabricator integration or Bitbucket Server plugin. This value is the space-separated list of allowed origins for cross-origin HTTP requests to Sourcegraph. Usually it contains the base URL for your Phabricator or Bitbucket Server instance.\n\nPreviously, this value was also used for the GitHub, GitLab, etc., integrations using the browser extension. It is no longer necessary for those. You may remove this setting if you are not using the Phabricator integration or Bitbucket Server plugin. eg \"https://my-phabricator.example.com https://my-bitbucket.example.com\"",
      "type": "string",
      "examples": ["https://my-phabricator.example.com https://my-bitbucket.example.com"],
      "group": "Security"
    },
    "lsifVerificationGithubToken": {
      "description": "The GitHub token that is used to verify that a user owns a repository.",
      "type": "string",
      "group": "Security"
    },
    "lsifUploadSecret": {
      "description": "Used to generate LSIF upload tokens. Must be long (20+ bytes) to make offline brute-force attacks difficult.",
      "type": "string",
      "group": "Security"
    },
    "disableAutoGitUpdates": {
      "description": "Disable periodically fetching git contents for existing repositories.",
      "type": "boolean",
      "default": false,
      "group": "External services"
    },
    "disablePublicRepoRedirects": {
      "description": "Disable redirects to sourcegraph.com when visiting public repositories that can't exist on this server.",
      "type": "boolean",
      "group": "External services"
    },
    "git.cloneURLToRepositoryName": {
      "description": "JSON array of configuration that maps from Git clone URL to repository name. Sourcegraph automatically resolves remote clone URLs to their proper code host. However, there may be non-remote clone URLs (e.g., in submodule declarations) that Sourcegraph cannot automatically map to a code host. In this case, use this field to specify the mapping. The mappings are tried in the order they are specified and take precedence over automatic mappings.",
      "type": "array",
      "items": {
        "title": "CloneURLToRepositoryName",
        "description": "Describes a mapping from clone URL to repository name. The ` + "`" + `from` + "`" + ` field contains a regular expression with named capturing groups. The ` + "`" + `to` + "`" + ` field contains a template string that references capturing group names. For instance, if ` + "`" + `from` + "`" + ` is \"^../(?P<name>\\w+)$\" and ` + "`" + `to` + "`" + ` is \"github.com/user/{name}\", the clone URL \"../myRepository\" would be mapped to the repository name \"github.com/user/myRepository\".",
        "type": "object",
        "additionalProperties": false,
        "required": ["from", "to"],
        "properties": {
          "from": {
            "description": "A regular expression that matches a set of clone URLs. The regular expression should use the Go regular expression syntax (https://golang.org/pkg/regexp/) and contain at least one named capturing group. The regular expression matches partially by default, so use \"^...$\" if whole-string matching is desired.",
            "type": "string"
          },
          "to": {
            "description": "The repository name output pattern. This should use ` + "`" + `{matchGroup}` + "`" + ` syntax to reference the capturing groups from the ` + "`" + `from` + "`" + ` field.",
            "type": "string"
          }
        }
      },
      "group": "External services"
    },
    "githubClientID": {
      "description": "Client ID for GitHub.",
      "type": "string",
      "group": "Internal",
      "hide": true
    },
    "githubClientSecret": {
      "description": "Client secret for GitHub.",
      "type": "string",
      "group": "Internal",
      "hide": true
    },
    "gitMaxConcurrentClones": {
      "description": "Maximum number of git clone processes that will be run concurrently to update repositories.",
      "type": "integer",
      "default": 5,
      "group": "External services"
    },
    "repoListUpdateInterval": {
      "description": "Interval (in minutes) for checking code hosts (such as GitHub, Gitolite, etc.) for new repositories.",
      "type": "integer",
      "default": 1,
      "group": "External services"
    },
    "maxReposToSearch": {
      "description": "The maximum number of repositories to search across. The user is prompted to narrow their query if exceeded. Any value less than or equal to zero means unlimited.",
      "type": "integer",
      "default": -1,
      "group": "Search"
    },
    "parentSourcegraph": {
      "description": "URL to fetch unreachable repository details from. Defaults to \"https://sourcegraph.com\"",
      "type": "object",
      "additionalProperties": false,
      "properties": {
        "url": {
          "type": "string",
          "default": "https://sourcegraph.com"
        }
      },
      "group": "External services"
    },
    "auth.accessTokens": {
      "description": "Settings for access tokens, which enable external tools to access the Sourcegraph API with the privileges of the user.",
      "type": "object",
      "additionalProperties": false,
      "properties": {
        "allow": {
          "description": "Allow or restrict the use of access tokens. The default is \"all-users-create\", which enables all users to create access tokens. Use \"none\" to disable access tokens entirely. Use \"site-admin-create\" to restrict creation of new tokens to admin users (existing tokens will still work until revoked).",
          "type": "string",
          "enum": ["all-users-create", "site-admin-create", "none"],
          "default": "all-users-create"
        }
      },
      "default": {
        "allow": "all-users-create"
      },
      "examples": [
        {
          "allow": "site-admin-create"
        },
        { "allow": "none" }
      ],
      "group": "Security"
    },
    "branding": {
      "description": "Customize Sourcegraph homepage logo and search icon.\n\nOnly available in Sourcegraph Enterprise.",
      "type": "object",
      "additionalProperties": false,
      "properties": {
        "light": {
          "$ref": "#/definitions/BrandAssets"
        },
        "dark": {
          "$ref": "#/definitions/BrandAssets"
        },
        "favicon": {
          "description": "The URL of the favicon to be used for your instance. We recommend using the following file format: ICO",
          "type": "string",
          "format": "uri"
        },
        "disableSymbolSpin": {
          "description": "Prevents the icon in the top-left corner of the screen from spinning on hover.",
          "type": "boolean",
          "default": false
        },
        "brandName": {
          "description": "String to display everywhere the brand name should be displayed. Defaults to \"Sourcegraph\"",
          "type": "string",
          "default": "Sourcegraph"
        }
      },
      "examples": [
        {
          "favicon": "https://example.com/favicon.ico",
          "light": {
            "logo": "https://example.com/logo_light.png",
            "symbol": "https://example.com/search_symbol_light_24x24.png"
          },
          "dark": {
            "logo": "https://example.com/logo_dark.png",
            "symbol": "https://example.com/search_symbol_dark_24x24.png"
          },
          "disableSymbolSpin": true
        }
      ]
    },
    "email.smtp": {
      "title": "SMTPServerConfig",
      "description": "The SMTP server used to send transactional emails (such as email verifications, reset-password emails, and notifications).",
      "type": "object",
      "additionalProperties": false,
      "required": ["host", "port", "authentication"],
      "properties": {
        "host": {
          "description": "The SMTP server host.",
          "type": "string"
        },
        "port": {
          "description": "The SMTP server port.",
          "type": "integer"
        },
        "username": {
          "description": "The username to use when communicating with the SMTP server.",
          "type": "string"
        },
        "password": {
          "description": "The username to use when communicating with the SMTP server.",
          "type": "string"
        },
        "authentication": {
          "description": "The type of authentication to use for the SMTP server.",
          "type": "string",
          "enum": ["none", "PLAIN", "CRAM-MD5"]
        },
        "domain": {
          "description": "The HELO domain to provide to the SMTP server (if needed).",
          "type": "string"
        }
      },
      "default": null,
      "examples": [
        {
          "host": "smtp.example.com",
          "port": 465,
          "username": "alice",
          "password": "mypassword",
          "authentication": "PLAIN"
        }
      ],
      "group": "Email"
    },
    "email.imap": {
      "title": "IMAPServerConfig",
      "description": "Optional. The IMAP server used to retrieve emails (such as code discussion reply emails).",
      "type": "object",
      "additionalProperties": false,
      "required": ["host", "port"],
      "properties": {
        "host": {
          "description": "The IMAP server host.",
          "type": "string"
        },
        "port": {
          "description": "The IMAP server port.",
          "type": "integer"
        },
        "username": {
          "description": "The username to use when communicating with the IMAP server.",
          "type": "string"
        },
        "password": {
          "description": "The username to use when communicating with the IMAP server.",
          "type": "string"
        }
      },
      "default": null,
      "examples": [
        {
          "host": "imap.example.com",
          "port": 993,
          "username": "alice",
          "password": "mypassword"
        }
      ],
      "group": "Email",
      "hide": true
    },
    "email.address": {
      "description": "The \"from\" address for emails sent by this server.",
      "type": "string",
      "format": "email",
      "group": "Email",
      "default": "noreply@sourcegraph.com"
    },
    "extensions": {
      "description": "Configures Sourcegraph extensions.",
      "type": "object",
      "properties": {
        "disabled": {
          "description": "Disable all usage of extensions.",
          "type": "boolean",
          "default": false,
          "!go": { "pointer": true }
        },
        "remoteRegistry": {
          "description": "The remote extension registry URL, or ` + "`" + `false` + "`" + ` to not use a remote extension registry. If not set, the default remote extension registry URL is used.",
          "oneOf": [{ "type": "string", "format": "uri" }, { "type": "boolean", "const": false }]
        },
        "allowRemoteExtensions": {
          "description": "Allow only the explicitly listed remote extensions (by extension ID, such as \"alice/myextension\") from the remote registry. If not set, all remote extensions may be used from the remote registry. To completely disable the remote registry, set ` + "`" + `remoteRegistry` + "`" + ` to ` + "`" + `false` + "`" + `.\n\nOnly available in Sourcegraph Enterprise.",
          "type": "array",
          "items": {
            "type": "string"
          }
        }
      },
      "default": {
        "remoteRegistry": "https://sourcegraph.com/.api/registry"
      },
      "examples": [
        {
          "remoteRegistry": "https://sourcegraph.com/.api/registry",
          "allowRemoteExtensions": ["sourcegraph/java"]
        }
      ],
      "group": "Extensions"
    },
    "discussions": {
      "description": "Configures Sourcegraph code discussions.",
      "type": "object",
      "properties": {
        "abuseProtection": {
          "description": "Enable abuse protection features (for public instances like Sourcegraph.com, not recommended for private instances).",
          "type": "boolean",
          "default": false
        },
        "abuseEmails": {
          "description": "Email addresses to notify of e.g. new user reports about abusive comments. Otherwise emails will not be sent.",
          "type": "array",
          "items": { "type": "string" },
          "default": []
        }
      },
      "group": "Experimental",
      "hide": true
    }
  },
  "definitions": {
    "BrandAssets": {
      "type": "object",
      "properties": {
        "logo": {
          "description": "The URL to the image used on the homepage. This will replace the Sourcegraph logo on the homepage. Maximum width: 320px. We recommend using the following file formats: SVG, PNG",
          "type": "string",
          "format": "uri"
        },
        "symbol": {
          "description": "The URL to the symbol used as the search icon. Recommended size: 24x24px. We recommend using the following file formats: SVG, PNG, ICO",
          "type": "string",
          "format": "uri"
        }
      }
    }
  }
}
`
