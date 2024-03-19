# Modeling teams in Sourcegraph

<aside class="experimental">
<p>
<span class="badge badge-experimental">Experimental</span> This feature is experimental and might change in the future.
</p>

<p><b>We're very much looking for input and feedback on this feature.</b> You can either <a href="https://sourcegraph.com/contact">contact us directly</a>, <a href="https://github.com/sourcegraph/sourcegraph">file an issue</a>, or <a href="https://twitter.com/sourcegraph">tweet at us</a>.</p>
</aside>

Teams in Sourcegraph are groups of users with a common handle. Teams are structured as a tree, so teams can have child teams.

Example team structure that can be modeled:

```
Engineering
├─ Security
├─ Code Graph
│  ├─ Batch Changes
│  ├─ Code Insights
├─ source
│  ├─ Repo Management
│  ├─ IAM
Product
```

Teams in Sourcegraph are usable in [code ownership](../../own/index.md), and other features in the future. Teams can be code owners and will influence the code ownership experience. You can search for code owned by a specific team, and in the future advanced ownership analytics will be informed by given team structures.

## Configuring teams

Teams can either be defined directly in Sourcegraph, or be ingested from external systems into Sourcegraph using [src-cli](https://github.com/sourcegraph/src-cli). A team name must be globally unique, and the global namespace for names is shared among users, teams, and orgs.

### From the UI

Go to **Teams** from the user navbar item. On this page, click "Create team". A team needs to have a unique name and can optionally take a display name. Additionally, you can add a parent team to build a tree structure as outlined above.

After hitting create, you will be redirected to the team page where you can add Sourcegraph users as team members.

> NOTE: It's common to define teams in Sourcegraph from a third party system. Teams defined from src-cli using the `-read-only` flag cannot be modified from the UI to prevent state drift from external systems.

### From the CLI

If you prefer a command line based approach, or would like to integrate an external system of record for teams into Sourcegraph, [src-cli](https://github.com/sourcegraph/src-cli) (v5.0+) provides commands to manage teams:

```bash
# List configured teams. Lists root teams, using -parent-team can read child teams.
src teams list [-query=<query>] [-parent-team=<name>]

# Create a new team.
src teams create -name=<name> [-display-name=<displayName>] [-parent-team=<name>] [-read-only]

# Update a team.
src teams update -name=<name> [-display-name=<displayName>] [-parent-team=<name>]

# Delete a team.
src teams delete -name=<name>

# List team members.
src teams members list -name=<name> [-query=<query>]

# Add a new team member. See user account matching for details on how this works.
src teams members add -team-name=<name> [-email=<email>] [-username=<username>] [-id=<ID>] [-external-account-service-id=<serviceID> -external-account-service-type=<serviceType> [-external-account-account-id=<accountID>] [-external-account-login=<login>]] [-skip-unmatched-members]

# Remove a team member. See user account matching for details on how this works.
src teams members remove -team-name=<name> [-email=<email>] [-username=<username>] [-id=<ID>] [-external-account-service-id=<serviceID> -external-account-service-type=<serviceType> [-external-account-account-id=<accountID>] [-external-account-login=<login>]] [-skip-unmatched-members]
```

#### User account matching

Matching a user account in Sourcegraph from an external system can be achieved in a few different ways: Sourcegraph User ID, Sourcegraph account email, Sourcegraph username or an explicit external-account mapping can be provided.

The matching order is as follows:
- try Sourcegraph user ID
- then try email
- then try username
- then try external-account

Example for external account matching with configured GitHub auth provider:

```bash
# Match a user with the account ID 123123123:
src teams members add \
  -team-name='engineering' \
  -external-account-service-id='https://github.com/' \
  -external-account-service-type='github' \
  -external-account-account-id='123123123'
# Match a user with the GitHub login handle alice:
src teams members add \
  -team-name='engineering' \
  -external-account-service-id='https://github.com/' \
  -external-account-service-type='github' \
  -external-account-login='alice'
```

### Permissions in teams

For now, team permissions are based on membership. Read-only teams are only editable by site-admins. The creator of a team can always modify it, even if they are not a member of it.

**Action**|**Site-admin**|**Regular user**|**Direct team member**
:-----:|:-----:|:-----:|:-----:
Reading teams, metadata and members|🟢|🟢|🟢
Creating a new team|🟢|🟢|n/a
Creating a new child team|🟢|🔴|🟢
Creating a new read-only team|🟢|🔴|n/a
Updating team details/metadata|🟢|🔴|🟢
Deleting a team|🟢|🔴|🟢
Deleting a read-only team|🟢|🔴|🔴
Adding a member to a team|🟢|🔴|🟢
Removing a member from a team|🟢|🔴|🟢
Adding a member to a read-only team|🟢|🔴|🔴
Removing a member from a read-only team|🟢|🔴|🔴

### Known limitations

- Read-only teams can only be created by site-admins
- Identity Provider / SCIM integrations are not available at the moment

## Common integrations

### GitHub teams

Using the GitHub CLI along with Sourcegraph's CLI, you can ingest teams data from GitHub into Sourcegraph. You may want to run this process regularly.

```bash
#!/usr/bin/env bash

set -e

ORG=<YOUR_ORG_NAME>
export ORG 

if [[ -z "${ORG}" ]]; then
  echo "ORG environment variable is required."
  exit 1
fi

SRC_ENDPOINT=<YOUR_SOURCEGRAPH_INSTANCE>
export SRC_ENDPOINT

if [[ -z "${GITHUB_TOKEN}" ]]; then
  echo "GITHUB_TOKEN environment variable is required."
  exit 1
fi

if [[ -z "${SRC_ACCESS_TOKEN}" ]]; then
  echo "SRC_ACCESS_TOKEN environment variable is required."
  exit 1
fi

# get_json_property parses the first argument string as JSON and returns the
# path passed as the second argument. Empty strings and null are truncated.
function get_json_property() {
  val="$(jq -r ".${2} | select (.!=null)" <<<"${1}")"
  if [[ -z "$val" || "$val" == "null" ]]; then
    echo -n
    return
  fi
  echo -n "$val"
}

# fetch_teams_paginated reads teams from the GitHub API in the configured organization.
# It reads all teams until pagination indicates all results have been fetched.
function fetch_teams_paginated() {
  query=$(cat <<EOF
    query(\$endCursor: String) {
      organization(login: "${ORG}") {
        teams(first: 100, after: \$endCursor) {
          nodes {
            name
            slug
            parentTeam {
              slug
            }
          }
          pageInfo {
            hasNextPage
            endCursor
          }
        }
      }
    }  
EOF
  )
  res=$(gh api graphql --paginate -f query="${query}")
  readarray -t p <<< "$(jq -c '.data.organization.teams.nodes[]' <<<"$res")"
  teams+=("${p[@]}")

  printf '%s\n' "${teams[@]}"
}

# fetch_team_members_paginated reads members of a team with the slug $1 from the
# GitHub API in the configured organization.
# It reads all members until pagination indicates all results have been fetched.
function fetch_team_members_paginated() {
  team_slug="$1"
  query=$(cat <<EOF
    query(\$endCursor: String) {
      organization(login: "${ORG}") {
        team(slug: "${team_slug}") {
          members(membership: IMMEDIATE, first: 100, after: \$endCursor) {
            nodes {
              databaseId
              login
            }
            pageInfo {
              hasNextPage
              endCursor
            }
          }
        }
      }
    }
EOF
  )
  res=$(gh api graphql --paginate -f query="${query}")
  readarray -t members <<< "$(jq -c '.data.organization.team.members.nodes[]' <<<"$res")"

  printf '%s\n' "${members[@]}"
}

# create_team_members attempts to leniently create team members for the given team $1.
# Team members are matched by account ID.
# This is a purely additive function, meaning members that are no longer part of a team
# will not be removed.
function create_team_members() {
  team="$1"

  readarray -t team_members < <(fetch_team_members_paginated "${team}")

  for member in "${team_members[@]}"
  do
    if [ -z "$member" ]; then
      continue
    fi
    echo "$(get_json_property "${member}" "login") is a member of ${team}"
    src teams members add \
      -skip-unmatched-members \
      -team-name="${team}" \
      -external-account-service-type=github \
      -external-account-service-id=https://github.com/ \
      -external-account-account-id="$(get_json_property "${member}" "databaseId")"
  done
}

# create_team creates a team for the GitHub JSON representation $1 of it.
# create_team then calls itself recursively for all of the teams child teams.
function create_team() {
  team="$1"
  function get_team_property() {
    get_json_property "${team}" "${1}"
  }
  
  name="$(get_team_property 'slug')"
  display_name="$(get_team_property 'name')"
  echo -n "Creating team ${name}"

  parent="$(get_team_property 'parentTeam.slug')"
  if [[ -n "$parent" ]]; then
    echo -n " (child of ${parent})"
  fi
  # Newline.
  echo

  set +e
  src team create -name="${name}" -display-name="${display_name}" -parent-team="${parent}" -read-only
  exit_code="$?"
  set -e
  # If team already exists, update instead.
  if [[ "$exit_code" == "3" ]]; then
    echo "Updating existing team ${name}"
    set +e
    src team update -name="${name}" -display-name="${display_name}" -parent-team="${parent}"
    exit_code="$?"
    set -e
    if [[ "${exit_code}" != "0" ]]; then
      echo "Failed to update team ${name}, skipping"
    else
      create_team_members "${name}"
    fi
  elif [[ "$exit_code" != "0" ]]; then
    exit "$exit_code"
  else
    create_team_members "${name}"
  fi

  # Create child teams.
  readarray -t child_teams < <(jq -c ". | select(.parentTeam.slug == \"${name}\")" <<<"${all_teams[@]}")
  for team in "${child_teams[@]}"
  do
    create_team "$team"
  done
}

# First, fetch all teams in the organization.
readarray -t all_teams < <(fetch_teams_paginated)

# Then, extract the root teams (ie. those without a parent).
readarray -t root_teams < <(jq -c '. | select(.parentTeam == null)' <<<"${all_teams[@]}")

# Recursively call create_team, starting at the root teams.
for team in "${root_teams[@]}"
do
  create_team "$team"
done
```

#### Known limitations

- This script does not remove teams that are no longer present in GitHub
- This script needs to run regularly as new users join Sourcegraph, to add them to the correct teams
- This script does not remove team members that are no longer part of a team, if they were added to the team in Sourcegraph before

### GitLab teams

Using the GitLab API, you can ingest teams data from GitLab into Sourcegraph. You may want to run this process regularly.

> NOTE: GitLab teams are not globally unique in name, only within their parent team. This is different to how teams work in Sourcegraph, where names are globally unique. You have to choose globally unique names when ingesting GitLab teams. This can affect name matching in code ownership.

```
TODO: Script here that scrapes the GitLab API for teams and converts them into Sourcegraph teams.
```
