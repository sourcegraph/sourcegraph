
This page contains information about releasing browser extensions for different browsers and creating your developer accounts.

# Releasing Browser Extensions
## Chrome
Deployment to the Chrome web store happen automatically in CI when the `bext/release` branch is updated.

## Firefox
Deployment to the Mozilla store happen automatically in CI when the `bext/release` branch is updated.

## Safari
The release process for Safari is currently not automated.

**!! Before you start !!**
- Navigate to https://appstoreconnect.apple.com/apps.
- You should see Sourcegraph for Safari.
   - If not, you should first ask a team member to add you to our Apple Developer group. They can send you an invitation to create an account.
- Once you created a Sourcegraph associated developer account, make sure you add this account to Xcode accounts.

Steps:
1. On your terminal navigate to `./sourcegraph/client/browser`.
1. Run the command `pnpm run build`.
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
1. Make sure upload is selected and click `next`.
1. Choose `SOURCEGRAPH INC` from the dropdown and click `next`.
1. Make sure uploading the symbols is checked and click `next`.
1. Make sure automatically managing the signing is checked and click `next`.
1. Validate everything on the summary page and click `upload`
1. Once successful, you can navigate to the app store connect webpage and see a new version being processed on the `Mac Build Activity` tab.
1. Once processing is done, navigate to `App Store` tab and click on the blue + symbol, located next to the `macOS App` label.
1. Enter the version number we've previously used on step 6 and click `create`.
1. A new version will appear on the left menu. Click on this new version and fill out the information textbox with a summary of updates.
1. Scroll down to the build section and click on the blue + symbol.
1. Select the build we've just uploaded and click done. (ignore compliance warning)
   1. Since our app communicates using https, we qualify for the export compliance. Select `Yes` and click `next`.
   1. Our use of encryption is exempt from regulations. Select `Yes` and click `next`.
1. We can now click the `Save` button and `Submit for Review`.

# Creating developer accounts for browser extensions
## Firefox
- Create an account on https://addons.mozilla.org/en-US/developers/
- Once account is created, invite to Sourcegraph
- An email confirmation will be sent
- Once the the account has been created, navigate to this website and remove yourself from listed authors (https://addons.mozilla.org/en-US/developers/addon/sourcegraph-for-firefox/ownership)

## Safari
- Ask a team member to add you to our Apple Developer group. They can send you an invitation from (App Store Connect)[https://appstoreconnect.apple.com/] portal.

## Chrome
- Be part of google group. Google group has access to Chrome Web Store Developer
Group for Chrome publishing permissions: sg-chrome-ext-devs@googlegroups.com
- Go to https://chrome.google.com/webstore/devconsole/register?hl=en
