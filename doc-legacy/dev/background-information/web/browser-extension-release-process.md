# Browser Extension Release Process

## Documentation

- [Browser Extension Release Process](#browser-extension-release-process)
  - [Documentation](#documentation)
  - [Creating developer accounts for browser extensions](#creating-developer-accounts-for-browser-extensions)
    - [Chrome](#chrome)
    - [Firefox](#firefox)
    - [Safari](#safari)
  - [Testing Checklist](#testing-checklist)
  - [Releasing Browser Extensions](#releasing-browser-extensions)
    - [Chrome](#chrome-1)
    - [Firefox](#firefox-1)
    - [Safari](#safari-1)
    - [Add status message to slack for public visibility for the org + history.](#add-status-message-to-slack-for-public-visibility-for-the-org--history)

## Creating developer accounts for browser extensions

Before releasing the browser extensions, you need to create developer accounts for the respecting platforms.

### <span id="create-for-chrome">Chrome</span>

1. Join to our sg-chrome-ext-devs [google group](https://groups.google.com/g/sg-chrome-ext-devs/).
1. Register a new account on the [Chrome Web Store](https://chrome.google.com/webstore/devconsole/register?hl=en). This step might require a small fee to register, you can expense this fee using the team budget.

### <span id="create-for-firefox">Firefox</span>

1. Create an account on the [Add-on developer hub](https://addons.mozilla.org/en-US/developers/).
1. Once the account is created, ask for a teammate to invite you to the Sourcegraph Org.
1. An email confirmation will be sent.
1. Once the the account has been confirmed, navigate to the ownership website and remove yourself from [listed authors](https://addons.mozilla.org/en-US/developers/addon/sourcegraph-for-firefox/ownership).

### <span id="create-for-safari">Safari</span>

1. Ask a team member to add you to our Apple Developer group. They can send you an invitation from [App Store Connect](https://appstoreconnect.apple.com/) portal.

## Testing Checklist

- [ ] Manually test installation on browsers
  - [ ] [Chrome](https://github.com/sourcegraph/sourcegraph/tree/main/client/browser#chrome)
  - [ ] [Firefox](https://github.com/sourcegraph/sourcegraph/tree/main/client/browser#firefox-manual)
  - [ ] [Safari](https://github.com/sourcegraph/sourcegraph/tree/main/client/browser#safari)
- [ ] Run browser extension e2e tests: `sg test bext-build && sg test bext-e2e`
  - > Note: it will automatically run anyway before releasing from the `bext/release` branch, but just to make sure before actual pushing to release branch.

## Releasing Browser Extensions

### Chrome

The release process for Chrome is fully automated. The review process usually takes between half a day to a day. To check the status of the release, visit the [developer dashboard](https://chrome.google.com/webstore/devconsole/7db1c88c-79ec-48c8-b14f-e17af93aee2c). Deployment to the Chrome web store happen automatically in CI when the `bext/release` branch is updated.

Release Steps:

1. Make sure the main branch is up-to-date. Run `git push origin main:bext/release`.
1. Pushing to the `bext/release` branch will trigger our build pipeline, which can be observed from respecting [buildkite](https://buildkite.com/sourcegraph/sourcegraph/builds?branch=bext%2Frelease) page.
1. Once the <code>🚀<img src="https://buildkiteassets.com/emojis/img-buildkite-64/chrome.png" style="width: 1.23em; height: 1.23em; margin-left: 0.05em; margin-right: 0.05em; vertical-align: -0.2em; background-color: transparent;"/> Extension release</code> task is done, the build should appear on the [developer dashboard](https://chrome.google.com/webstore/devconsole/7db1c88c-79ec-48c8-b14f-e17af93aee2c/dgjhfomjieaadpoljlnidmbgkdffpack/edit/package) with pending review status.

### Firefox

The release process for Firefox is currently semi-automated. The review process can take between half a day to multiple days. To check the status of the release, visit the [add-on developer hub](https://addons.mozilla.org/en-US/developers/addon/sourcegraph-for-firefox/versions).

Release Steps:

1. When the `bext/release` branch is updated, our build pipeline will trigger a build for the Firefox extension (take a note of the commit sha, we'll need it later).
1. The buildkite will, similar to Chrome, run a task named <code>🚀<img src="https://buildkiteassets.com/emojis/img-buildkite-64/firefox.png" style="width: 1.23em; height: 1.23em; margin-left: 0.05em; margin-right: 0.05em; vertical-align: -0.2em; background-color: transparent;"/> Extension release</code>.
1. Once the release task is finished check the [add-on developer hub](https://addons.mozilla.org/en-US/developers/addon/sourcegraph-for-firefox/versions).
   1. If the currently build version is available and in "Approved" state then we are done.
   1. If it is in other state, then:
      1. We need to upload an non-minified version of the extension to the add-on developer hub.
      1. To create this non-minified package, on your local git copy, navigate to `sourcegraph/client/browser/scripts/`, open the file `create-source-zip.js`, and modify the `commitId` variable (use the sha from earlier).
      1. Once the variable is modified, run this script by `node create-source-zip.js`. It will generate a `sourcegraph.zip` in the folder.
      1. Navigate to the [add-on developer hub](https://addons.mozilla.org/en-US/developers/addon/sourcegraph-for-firefox/versions), click on the pending version, upload the zip that was just created and `Save Changes`.

### Safari

The release process for Safari is currently not automated. The review process usually takes between half a day to a day. To check the status of the release, visit the [App Store Connect](https://appstoreconnect.apple.com/apps/1543262193/appstore/macos/version/deliverable).
Steps:

1. On your terminal and run the command `pnpm --filter @sourcegraph/browser build`.
1. Build will generate an Xcode project under `./sourcegraph/client/browser/build/Sourcegraph for Safari`.
   1. If you run into Xcode related errors, make sure that you've downloaded Xcode from the app store, opened it and accepted the license/terms agreements.
1. Open the project using Xcode.
1. Navigate to the General settings tab.
1. Select the target `Sourcegraph for Safari`.
   1. Change `App Category` to `Developer Tools`.
   1. Increment the `Version` & `Build` numbers. You can find the current numbers on the [App Store Connect page](https://appstoreconnect.apple.com/apps/1543262193/appstore/macos/version/deliverable).
1. Select the target `Sourcegraph for Safari Extension`.
   1. Increment the `Version` & `Build` numbers. You can find the current numbers on the [App Store Connect page](https://appstoreconnect.apple.com/apps/1543262193/appstore/macos/version/deliverable).
1. Open `Assets.xcassets` from the file viewer and select `AppIcon`. We need to upload the 512x512px & 1024x1024px version icons for the Mac Store. Drag & drop the files from [Drive](https://drive.google.com/drive/folders/1JCUuzIrpNrZP_uNqpel2wq0lwdRBkVgZ) to the corresponding slots.
1. On the menu bar, navigate to `Product > Achive`. Once successful, the Archives modal will appear. If you ever want to re-open this modal, you can do so by navigating to the `Window > Organizier` on the menu bar.
1. With the latest build selected, click on the `validate` button.
1. Choose `SOURCEGRAPH INC` from the dropdown and click `next`.
1. Make sure uploading the symbols is checked and click `next`.
1. Make sure automatically managing the signing is checked and click `next`.
   1. If this is your first time signing the package, you need to create your own local distribution key.
1. Once the validation is complete, click on the `Distribute App`.
1. Make sure `App Store Connect` is selected and click `next`.
1. Make sure `Upload` is selected and click `next`.
1. Choose `SOURCEGRAPH INC` from the dropdown and click `next`.
1. Make sure uploading the symbols is checked and click `next`.
1. Make sure automatically managing the signing is checked and click `next`.
1. Validate everything on the summary page and click `upload`
1. Once successful, you can navigate to the [App Store Connect webpage](https://appstoreconnect.apple.com/apps/1543262193/testflight/macos) and see a new version being processed.
1. Once processing is done, navigate to `App Store` tab and click on the blue + symbol, located next to the `macOS App` label.
1. Enter the version number we've previously used on step 6 and click `create`.
1. A new version will appear on the left menu. Click on this new version and fill out the information textbox with a summary of updates.
1. Scroll down to the build section and click on the blue + symbol.
1. Select the build we've just uploaded and click done. (ignore compliance warning)
   1. Since our app communicates using https, we qualify for the export compliance. Select `Yes` and click `next`.
   1. Our use of encryption is exempt from regulations. Select `Yes` and click `next`.
1. We can now click the `Save` button and `Submit for Review`.

### Add status message to slack for public visibility for the org + history.

1. Create a PR with updates for `client/browser/CHANGELOG.md`. See [example commit](https://github.com/sourcegraph/sourcegraph/commit/2683fae5671de24b2f8dda3504dac40904f9f913)
1. Add a message to #integrations slack channel, with updates in a thread on release updates. See [example message](https://sourcegraph.slack.com/archives/C01LZKLRF0C/p1637851520182400)
