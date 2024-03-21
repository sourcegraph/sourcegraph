# Changelog

The Sourcegraph extension uses major.EVEN_NUMBER.patch (eg. 2.0.1) for release versions and major.ODD_NUMBER.patch (eg.
2.1.1) for pre-release versions.

## Unreleased

### Changes

- Change the extension name from "Sourcegraph" to "Search by
  Sourcegraph" [pull/51790](https://github.com/sourcegraph/sourcegraph/pull/51790)

### Fixes

- Various UI fixes for dark and light themes [pull/50598](https://github.com/sourcegraph/sourcegraph/pull/50598)

## 2.2.15

### Fixes

- Prefer `upstream` and `origin` remotes when no remote is
  selected [issues/2761](https://github.com/sourcegraph/sourcegraph/issues/2761) [pull/48369](https://github.com/sourcegraph/sourcegraph/pull/48369)
- Fixes content security policy configuration: [pull/47263](https://github.com/sourcegraph/sourcegraph/pull/47263)

## 2.2.14

### Changes

- Implement proxy support in the process that has access to node
  APIs [issues/41181](https://github.com/sourcegraph/sourcegraph/issues/41181)

## 2.2.13

### Changes

- Support for logical multiline matches in the UI for Sourcegraph instance versions >=
  3.42.0 [pull/43007](https://github.com/sourcegraph/sourcegraph/pull/43007)
- Tokens will now be stored in secret storage and removed from user
  settings [issues/36731](https://github.com/sourcegraph/sourcegraph/issues/36731)
- Users can now log in through the
  built-in [authentication API](https://code.visualstudio.com/api/references/vscode-api#authentication) [issues/36731](https://github.com/sourcegraph/sourcegraph/issues/36731)
- Add log out button to `Help and Feedback` sidebar
  under `User` [issues/36731](https://github.com/sourcegraph/sourcegraph/issues/36731)

### Fixes

- Fix issue where pattern type was always set to `literal` for Sourcegraph instance versions earlier than v3.43.0, which
  was overriding regex/structural toggles [pull/43005](https://github.com/sourcegraph/sourcegraph/pull/43005)

## 2.2.12

### Fixes

- Vary search pattern type depending on Sourcegraph instance
  version: [issues/41236](https://github.com/sourcegraph/sourcegraph/issues/41236), [pull/42178](https://github.com/sourcegraph/sourcegraph/pull/42178)

## 2.2.10

### Changes

- Remove tracking parameters from all shareable URLs [pull/42022](https://github.com/sourcegraph/sourcegraph/pull/42022)

### Fixes

- Fix Sourcegraph blob link
  generation: [issues/42060](https://github.com/sourcegraph/sourcegraph/issues/42060), [pull/42065](https://github.com/sourcegraph/sourcegraph/pull/42065)

## 2.2.9

### Fixes

- Fix an issue that prevented search results on some older Sourcegraph instance versions to not render
  properly [pull/40621](https://github.com/sourcegraph/sourcegraph/pull/40621)

## 2.2.8

### Changes

- `Internal:` Automate release step for Open-VSX
  registry: [issues/37704](https://github.com/sourcegraph/sourcegraph/issues/37704)
- Remove integrations banners and corresponding
  pings: [issues/38625](https://github.com/sourcegraph/sourcegraph/issues/38625), [pull/38715](https://github.com/sourcegraph/sourcegraph/pull/38715), [pull/38862](https://github.com/sourcegraph/sourcegraph/pull/38862)

### Fixes

## 2.2.7

### Changes

- Remove references to creating an account on cloud, or configuring a cloud
  account [pull/38071](https://github.com/sourcegraph/sourcegraph/pull/38071)

### Fixes

## 2.2.6

### Changes

- Remove notification to add Sourcegraph extension to the
  workspace [issues/37772](https://github.com/sourcegraph/sourcegraph/issues/37772)

### Fixes

-

## 2.2.5

### Changes

- Update Sourcegraph logo in sidebar [issues/37710](https://github.com/sourcegraph/sourcegraph/issues/37710)
- Sourcegraph extension is now listed
  in [Open VSX Registry](https://open-vsx.org/extension/sourcegraph/sourcegraph) [issues/36477](https://github.com/sourcegraph/sourcegraph/issues/36477)
- Sourcegraph extension is now available for installation in all Gitpod VS Code
  Workspaces [issues/37760](https://github.com/sourcegraph/sourcegraph/issues/37760)

## 2.2.4

### Changes

- Optimize package size [issues/36192](https://github.com/sourcegraph/sourcegraph/issues/36192)

### Fixes

- Check if default branch exists when opening
  files [issues/36743](https://github.com/sourcegraph/sourcegraph/issues/36743)

## 2.2.3

### Changes

- Update Access Token headers setting method --thanks @ptxmac for the
  contribution! [issues/34338](https://github.com/sourcegraph/sourcegraph/issues/34338)
- Add options to choose between main branch or current branch when copy/open file. Always use default branch if
  set [issues/34591](https://github.com/sourcegraph/sourcegraph/issues/34591)
- CTA for adding Sourcegraph extension to Workspace
  Recommendations [issues/34829](https://github.com/sourcegraph/sourcegraph/issues/34829)

### Fixes

- Windows file path issue [issues/34788](https://github.com/sourcegraph/sourcegraph/issues/34788)
- Sourcegraph icon in help sidebar now shows on light
  theme [issues/35672](https://github.com/sourcegraph/sourcegraph/issues/35672)
- Highlight background color for VS Code Light & Light+
  Theme [issues/35767](https://github.com/sourcegraph/sourcegraph/issues/35767)
- Display reload button when instance URL is
  updated [issues/35980](https://github.com/sourcegraph/sourcegraph/issues/35980)

## 2.2.2

### Changes

- Display current extension version and instance version in
  frontend [issues/34729](https://github.com/sourcegraph/sourcegraph/issues/34729)

### Fixes

- Remove incorrect unsupported instance error messages on first
  load [issues/34207](https://github.com/sourcegraph/sourcegraph/issues/34207)
- Links to open remote file in Sourcegraph web are now decoded
  correctly [issues/34630](https://github.com/sourcegraph/sourcegraph/issues/34630)
- Remove pattern restriction for basePath [issues/34731](https://github.com/sourcegraph/sourcegraph/issues/34731)

## 2.2.1

### Changes

- Add Help and Feedback sidebar [issue/31021](https://github.com/sourcegraph/sourcegraph/issues/31021)
- Add CONTRIBUTING guide [issue/26536](https://github.com/sourcegraph/sourcegraph/issues/26536)
- Display error message when connected to unsupported
  instances [issue/31808](https://github.com/sourcegraph/sourcegraph/issues/31808)
- Log events with `IDEEXTENSION` as event source for instances on 3.38.0 and
  above [issue/32851](https://github.com/sourcegraph/sourcegraph/issues/32851)
- Add new configuration setting:
  sourcegraph.basePath [issue/32633](https://github.com/sourcegraph/sourcegraph/issues/32633)
- Add ability to open local copy of a search result if file exists in current workspace or
  basePath [issue/32633](https://github.com/sourcegraph/sourcegraph/issues/32633)

### Fixes

- Improve developer scripts [issue/32741](https://github.com/sourcegraph/sourcegraph/issues/32741)
- Code Monitor button redirect issue for non signed-in
  users [issues/33631](https://github.com/sourcegraph/sourcegraph/issues/33631)
- Error regarding missing PatternType when creating save
  search [issues/31093](https://github.com/sourcegraph/sourcegraph/issues/31093)

## 2.2.0

### Changes

- Add pings for Sourcegraph ide extensions usage
  metrics [issue/29124](https://github.com/sourcegraph/sourcegraph/issues/29124)
- Add input fields to update Sourcegraph instance
  url [issue/31804](https://github.com/sourcegraph/sourcegraph/issues/31804)
- Clear search results on tab close [issue/30583](https://github.com/sourcegraph/sourcegraph/issues/30583)

## 2.0.9

### Changes

- Add Changelog for version tracking purpose [issue/28300](https://github.com/sourcegraph/sourcegraph/issues/28300)
- Add VS Code Web support for instances on
  3.36.0+ [issue/28403](https://github.com/sourcegraph/sourcegraph/issues/28403)
- Update to use API endpoint for stream search [issue/30916](https://github.com/sourcegraph/sourcegraph/issues/30916)
- Add new configuration setting `sourcegraph.requestHeaders` for adding custom
  headers [issue/30916](https://github.com/sourcegraph/sourcegraph/issues/30916)

### Fixes

- Manage context display issue for instances under
  v3.36.0 [issue/31022](https://github.com/sourcegraph/sourcegraph/issues/31022)

## 2.0.8

### Fixes

- Files will open in the correct url scheme [issue/31095](https://github.com/sourcegraph/sourcegraph/issues/31095)
- The 'All Search Keywords' button is now linked to Sourcegraph docs site
  correctly [issue/31023](https://github.com/sourcegraph/sourcegraph/issues/31023)
- Update Sign Up links with the correct utm parameters

## 2.0.7

### Changes

- Remove Sign Up CTA in Sidebar for self-host instances

### Fixes

- Add backward compatibility for configuration settings from v1: `sourcegraph.defaultBranch`
  and `sourcegraph.remoteUrlReplacements`

## 2.0.6

### Changes

- Remove Sign Up CTAs in Search Result for self-host instances

## 2.0.1

### Changes

- Add Code Monitor
