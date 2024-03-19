# Temporary settings

Basic user settings that should be retained for users across sessions
can be stored as temporary settings. These should be trivial settings
that would be fine if they were lost.

For authenticated users, temporary settings are stored in the database
in the `temporary_setings` table and are queried and modified via the GraphQL API.

For unauthenticated users, temporary settings are stored in `localStorage`.

## Difference between temporary settings, site settings, and localStorage

Site settings are the primary way to handle settings in Sourcegraph. They are accessible as
global site settings, org settings, and user settings. Meanwhile, localStorage is a way of 
storing settings directly in the browser. These are the primary differences between temporary 
settings, site settings, and localStorage:

|  | Site settings | localStorage | Temporary settings |
|---|---|---|---|
| User editable | ✅  | ❌ | ❌ |
| Cascades from global to org to users | ✅  | ❌ | ❌ |
| Persisted across browsers when user logs in | ✅  | ❌ | ✅ |
| Stored for unauthenticated users | ❌ <br /> (will use global site settings) | ✅ | ✅ |
| Typed schema | ✅  <br /> (in [`settings.schema.json`](https://sourcegraph.com/github.com/sourcegraph/sourcegraph/-/blob/schema/settings.schema.json))| ❌ | ✅  <br /> (in [`TemporarySettings.ts`](https://sourcegraph.com/github.com/sourcegraph/sourcegraph/-/blob/client/shared/src/settings/temporary/TemporarySettings.ts))|
| Available in Go code | ✅  | ❌ | ❌ |


## Examples

Examples of data that is a good candidate for temporary settings include:

* Settings that should be available to unauthenticated users
* The dismiss state of a modal
* The collapse state of a panel
* Basic theme settings like "light" or "dark"
* "Most recently used" lists
* Data needed for keeping track of a user's interactions as part of an
  A/B test or flight, or similar settings that should not be user-editable

Examples of data that should not be stored as temporary settings include:

* Any data that should not be retained between sessions, such as search results
* Any data that may need to be shared between users, such as a code insights chart
  or a search context configuration
* Data that may not be easily recoverable in a few clicks, such as a search notebook
* Settings that need to cascade from global site settings or org settings to users
  (temporary settings don't support cascading)
* Any settings the user would like to edit manually (temporary settings are not user-editable)
* Any data that is specific to one device/browser (temporary settings are synced between 
  devices for logged-in users; data that should not be synced should use localStorage)

## Using temporary settings

### Update schema

Update the interface [`TemporarySettingsSchema` in `TemporarySettings.ts`](https://sourcegraph.com/github.com/sourcegraph/sourcegraph/-/blob/client/shared/src/settings/temporary/TemporarySettings.ts?L8)
by adding a key for the setting you want to store. The key should be namespaced based on
the area of the site that will be using the settings. Example names include `'search.collapsedSidebarSections'`
or `'codeInsights.hiddenCharts'`. The value of the setting can be any JSON-serializable type.

### Getting and setting settings

Use the React hook [`useTemporarySetting`](https://sourcegraph.com/github.com/sourcegraph/sourcegraph/-/blob/client/shared/src/settings/temporary/useTemporarySetting.ts)
to get an up-to-date value of the setting and a function that can update the value,
similar to other hooks like `useState`. The value will be updated automatically if
the user's authentication state changes or the setting is modified elsewhere in the
application.

An optional second argument can be provided to specify a default value if the
setting hasn't been set yet.

NOTE: Settings might be loaded asynchronously, in which case the initial value
of the setting will be `undefined`. You might have to account for this in your
component to prevent a flash of content. The optional default value is only used
if the setting hasn't been set yet; it is *not* used as an initial value.

#### Example usage:

```typescript
const [modalVisible, setModalVisible] = useTemporarySetting('example.modalVisible')

const toggleModal = () => {
    setModalVisible(currentVisibility => !currentVisibility)
}

return <>
    {modalVisible && <Modal onClose={toggleModal} />}
</>
```

### 🚨 Data sync warning

Temporary settings for authenticated users are only updated every 5 minutes.
This can cause settings to become out-of-sync or lost if modified in more than
one tab/browser at once. **Do not use temporary settings for data that may not
be easily recoverable with a few clicks.**


## Viewing and modifying raw temporary settings

For debugging and testing purposes, it can be useful to view and modify the raw
temporary settings data. In particular, clearing temporary settings can help
emulate the flow of what a new user would see.

### Unauthenticated users

You can view and modify temporary settings using the `localStorage` in the browser
developer tools' Storage (Firefox & Safari) or Application (Chromium) tab, or by
calling `localStorage` directly from the console. Temporary settings are stored in
`localStorage` with the `temporarySettings` key. Deleting the item with this key will
clear all temporary settings.

Useful console commands:

```js
localStorage['temporarySettings'] // Get settings

localStorage.removeItem('temporarySettings') // Clear settings
```

### Authenticated users

You can view and modify temporary settings via the GraphQL API using the
[GraphQL console](https://sourcegraph.com/api/console).

You can view your temporary settings with the `temporarySettings` GraphQL query:

```graphql
query {
  temporarySettings {
    contents
  }
}
```

You can modify your temporary settings with the `overwriteTemporarySettings` GraphQL mutation.
For example, the following mutation will clear your temporary settings:

```graphql
mutation {
  overwriteTemporarySettings(contents: "{}") {
    alwaysNil
  }
}
```

## Testing temporary settings

In order to make it easier to write test against temporary settings, you can use the `<MockTemporarySettings>` component inside your component tests:

```typescript
it('mocks saved temporary settings', () => {
    render(
        <MockTemporarySettings settings={{ 'search.notepad.enabled': false }}>
            <MyComponent />
        </MockTemporarySettings>
    )

    // Your test assertions
})
```
