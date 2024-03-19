# TypeScript programming patterns

This is a non-exhaustive lists of programming patterns used across our TypeScript codebases.
Since patterns are used everywhere, they are usually not documented with inline code comments.
This document serves as the canonical place to document them.

This document is specifically intended for patterns that are hard to detect automatically with static analysis.
If possible, we generally prefer to encode our best practices in ESLint configuration.
For automatically detectable patterns, please see the documentation of our [ESLint configuration](https://github.com/sourcegraph/eslint-config#principles).
Our code is also formatted automatically by [Prettier](https://prettier.io/) so we don't waste time bike-shedding formatting.

Related pages: [Testing web code](../testing_web_code.md)

## `Subscription` bag

Functions, classes or React class components often register external subscriptions, usually with RxJS.
These Subscriptions must be cleaned up when the instance of the class is no longer in use,
but won't be automatically through garbage collection, because the external emitter holds a (transitive) reference to the class instance, not the other way around.

If the Subscriptions are not cleaned up, the garbage collector can not collect the class instance,
and the subscribe callback may still run code operating on invalid state.
In the case of React class components, it may call `setState()` after the component was unmounted,
which causes React to throw an Error.

If you forgot to handle a Subscription returned by Rx `observable.subscribe()`, the ESLint rule `rxjs/no-ignored-subscription` will warn you.

The easiest way to solve this is to save all Subscriptions in a Subscription bag, and when the instance is disposed, unsubscribe that bag.
[RxJS Subscriptions](https://rxjs-dev.firebaseapp.com/guide/subscription) have the nice property of being _composable_, meaning they can act as a Subscription bag.
You can add more Subscriptions to it with the `add()` method, and unsubscribe all at once by calling `unsubscribe()`.
It handles all the edge cases like adding a Subscription after it was already unsubscribed.

Subscriptions don't just accept other Subscriptions, they accept arbitrary cleanup functions to be run on unsubscription, so this pattern can also be used to clean up non-RxJS resources or listeners.

Example of what this pattern typically looks like in a React class component:

```ts
class MyComponent extends React.Component {
  // Subscription used as a Subscription bag
  private subscriptions = new Subscription()

  public componentDidMount(): void {
    // Register the Subscription in the Subscription bag
    this.subscriptions.add(
      someObservable.subscribe(value => {
        // This is guaranteed to never run after this.subscriptions.unsubscribe() was called
        this.setState({ value })
      })
    )
  }

  // React lifecycle method that gets run on disposal
  public componentWillUnmount(): void {
    // Unsubscribes all Subscriptions that were made
    this.subscriptions.unsubscribe()
  }
}
```

The Subscription bag pattern is usually not needed when using React function components with our [`useObservable()` family of hooks](https://sourcegraph.com/github.com/sourcegraph/sourcegraph/-/blob/client/shared/src/util/useObservable.ts#L26:17), as they will handle the subscription under the hood.

## Making invalid states impossible through union types

TypeScript has a very powerful type system, including support for [union types](https://www.typescriptlang.org/docs/handbook/advanced-types.html#union-types).
A union type `A | B | C` declares that the value can be either of type `A`, `B` or `C`.
Often times when thinking from the perspective of the state consumer, you end up with several different state slots that need to be represented.
For example, in a UI that fetches user data from the network, you need to show a loader when the data is loading, the contents if the result arrived, or an error if the fetch failed.
A naive approach to model this would look like this:

```ts
// BAD
{
  isLoading: boolean
  error?: Error
  user?: User
}
```

These three properties create a matrix with a total of 2<sup>3</sup> = 8 different possible states.
However, only 3 of these states are _valid_:
You want to either show the loader, the error **or** the contents, never any of them at the same time.
But by using three properties, it is possible that e.g. `isLoading` is `true`, while `user` is defined too.
The application needs to ensure that for any change of one of the properties, the other two are reset, e.g. `isLoading` is reset to `false` as soon as the contents are loaded, as well as `errorMessage`.

There are two bad effects of this:
- As the code evolves and grows more complex, these checks could easily be omitted, and the application could end up showing a loader and the contents at the same time.
- TypeScript has no way to know that `contents` is always defined when `isLoading` is not, so you'll have to either cast in multiple places or duplicate checks. The cast could then become invalid in the future and cause errors.

This can be easily avoided by expressing the mutually exclusive nature of the state slots in the type system through a union type:

```ts
// GOOD
{
  userOrError?: User | Error
}
```

Here, `undefined` represents _loading_, and a defined value means either a successful fetch **or** an error.
To find out which state is active, you can use type checking features:
- comparing to `undefined`, `null` or a defined constant with `===`
- using `typeof` if one of the states is a primitive type
- using `instanceof` if one of the states is a class
- checking a [discriminator property](https://www.typescriptlang.org/docs/handbook/advanced-types.html#discriminated-unions) like `type` or `kind` if available
- using a [custom type guard function](https://www.typescriptlang.org/docs/handbook/advanced-types.html#type-guards-and-differentiating-types) if you need to distinguish between completely different object interfaces

TypeScript is then able to narrow the type correctly in the conditional branches defined by `if`/`else`, the ternary operator, or `return`:

```ts
if (userOrError === undefined) {
  return <Loader />
}
if (userOrError instanceof Error) {
  return <ErrorAlert error={userOrError} />
}
return <div>Username: {userOrError.username}</div>
```

We are now relying on the type system to enforce at compile time that it is impossible to have a loader and an error shown at the same time.

**Caveat**: If you are using the `Error` type, make sure that the exception you are saving is actually an Error with our `asError()` utility.
Exceptions in TypeScript are of type `any`, i.e. if a function deep down exhibits the bad practice of throwing e.g. a string, it could mess up the type checking logic.
If you accidentally return an `any` typed error without using `asError()`, the ESLint rule `no-unsafe-return` will warn you.

## Avoiding classes

In traditional OOP programming languages, classes are often used for **encapsulation** and **polymorphism**.
In TypeScript, classes are not needed to achieve these goals.
Encapsulation can be easily achieved with modules (i.e. non-exported module members) and closures.
Polymorphism is available without classes thanks to duck-typed interfaces and object literals.

We found that when using classes as an encapsulation boundary in TypeScript, over time they often grow to violate the [Single Responsibility Principle](https://en.wikipedia.org/wiki/Single-responsibility_principle),  become hard to reason about and hard to test.
Methods often get added to the class that only access a subset of the properties of the class.
Splitting the class up afterwards into multiple smaller classes that draw better boundaries takes a lot of effort.

Starting out from the beginning with **individual functions** (instead of methods) that take **just the data they need** as interface-typed parameters (instead of accessing class fields) avoids this problem.
It makes it easy to evolve each function individually, increase or decrease their data dependencies, and split or merge them.
Ideally these functions do not mutate the input object, but each produce a new result object instead.
This makes it easy to compose them.
Avoid having functions that take more data than they actually need just to return this part of the input verbatim, because it makes them harder to test and ties them to the context they are used in currently.
This "merging" is better done by the caller.
Instead of constructors, factory functions can be defined that create and return object literals (typed with an interface).
These functions are conventionally named `create`+_name of interface_ (eg. [`createModelService()`](https://sourcegraph.com/github.com/sourcegraph/sourcegraph@b1ddeff4a2b94ceccda7cdf7021d5f82aa4522ed/-/blob/shared/src/api/client/services/modelService.ts#L99-167)).

There are a few places where we do use classes, e.g. [`ExtDocuments`](https://sourcegraph.com/github.com/sourcegraph/sourcegraph@fd9eef0b5893dfb3358d2a3358d15f3e9b14ca9e/-/blob/shared/src/api/extension/api/documents.ts#L21:20).
These are usually where mutation is unavoidable.
For example, our extension host web worker runs in a separate thread.
We need to sync various data between the worker and the main thread, because requesting that data on demand every time  through message passing would not be performant.

## Converting React class components to function components

We have a large number of React class components in our codebase predating [React hooks](https://reactjs.org/docs/hooks-intro.html).
We are continuously refactoring these to function components using hooks. This section provides examples of refactoring common class component patterns to into function components.

### `logViewEvent` calls

When refactoring, simply move these calls from `componentDidMount()` to a `useEffect()` callback:

```typescript
useEffect(() => {
    eventLogger.logViewEvent('ApiConsole')
}, [])
```

### Fetching data in componentDidMount

A lot of components will fetch initial data using observables, calling `setState()` in the `subscribe()` callback. See [this example from `SiteUsageExploreSection`](https://sourcegraph.com/github.com/sourcegraph/sourcegraph@9997438a8ba2fdc54920f1f6ad22dd08d4a37215/-/blob/web/src/usageStatistics/explore/SiteUsageExploreSection.tsx?subtree=true#L32-38):

```typescript
    public componentDidMount(): void {
        this.subscriptions.add(
            fetchSiteUsageStatistics()
                .pipe(catchError(error => [asError(error)]))
                .subscribe(siteUsageStatisticsOrError => this.setState({ siteUsageStatisticsOrError }))
        )
    }
```

In function components, this pattern can be easily replaced with `useObservable()`:

```typescript
const usageStatisticsOrError = useObservable(fetchSiteUsageStatistics().pipe(catchError(error => [asError(error)])))
```

If the function returning the Observable took props as parameters, you can rely on `useMemo()` to [re-create the Observable when props change](https://sourcegraph.com/github.com/sourcegraph/sourcegraph@9997438a8ba2fdc54920f1f6ad22dd08d4a37215/-/blob/web/src/enterprise/site-admin/SiteAdminLsifUploadPage.tsx?subtree=true#L25-27):

```typescript
const uploadOrError = useObservable(
    useMemo(() => fetchLsifUpload({ id }).pipe(catchError(error => [asError(error)])), [id])
)
```

### `this.componentUpdates`

A common pattern in our class components is to declare a Subject of the component's props, named `componentUpdates`. It can be used to trigger side-effects, such as fetching data or subscribing to an Observable passed as props, only when certain props change.

See this example from [`<MonacoEditor/>`](https://sourcegraph.com/github.com/sourcegraph/sourcegraph@9997438a8ba2fdc54920f1f6ad22dd08d4a37215/-/blob/web/src/components/MonacoEditor.tsx?subtree=true#L129-137):

```typescript
this.subscriptions.add(
    this.componentUpdates
        .pipe(
            map(({ isLightTheme }) => (isLightTheme ? SOURCEGRAPH_LIGHT : SOURCEGRAPH_DARK)),
            distinctUntilChanged()
        )
        .subscribe(theme => monaco.editor.setTheme(theme))
)
this.componentUpdates.next(this.props)
```

This can be easily refactored with hooks by passing the relevant property as a dependency:

```typescript
const theme = isLightTheme ? SOURCEGRAPH_LIGHT : SOURCEGRAPH_DARK
useEffect(() => {
    monaco.editor.setTheme(theme)
}, [theme])
```

### Instance property subjects

Some class components expose subjects as instance properties, that can be used to trigger a new fetch of the data. See [`SettingsArea.refreshRequests`](https://sourcegraph.com/github.com/sourcegraph/sourcegraph@9997438a8ba2fdc54920f1f6ad22dd08d4a37215/-/blob/web/src/settings/SettingsArea.tsx?subtree=true#L73):

```typescript
// Load settings.
this.subscriptions.add(
    combineLatest([
        this.componentUpdates.pipe(
            map(props => props.subject),
            distinctUntilChanged()
        ),
        this.refreshRequests.pipe(startWith<void>(undefined)),
    ])
        .pipe(
            switchMap(([{ id }]) =>
                fetchSettingsCascade(id).pipe(
                    switchMap(cascade =>
                        this.getMergedSettingsJSONSchema(cascade).pipe(
                            map(settingsJSONSchema => ({ subjects: cascade.subjects, settingsJSONSchema }))
                        )
                    ),
                    catchError(error => [asError(error)]),
                    map(dataOrError => ({ dataOrError }))
                )
            )
        )
        .subscribe(
            stateUpdate => this.setState(stateUpdate),
            error => console.error(error)
        )
)
```

These can typically be refactored using `useEventObservable()`, to handle both the initial data fetch, and further refresh requests:

```typescript
const [nextRefreshRequest, dataOrError] = useEventObservable(
    useCallback(
        (refreshRequests: Observable<void>) => refreshRequests.pipe(
            // Seed the pipe so that the initial fetch is triggered
            startWith<void>(undefined),
            switchMapTo(fetchSettingsCascade(props.subject.id)),
            switchMap(cascade => getMergedSettingsJSONSchema(cascade)),
            catchError(error => [asError(error) as ErrorLike])
        ),
        // Pass props.subject.id as dependency so that settings are fetched again
        // when the subject passed as props changes
        [props.subject.id]
    )
)
```

### Uses of `tap()` to trigger side-effects in Observable pipes

Some Observable pipes use `tap()` to trigger side-effects, such as [this pipe in `<RepoRevisionContainer/>`](https://sourcegraph.com/github.com/sourcegraph/sourcegraph@9997438a8ba2fdc54920f1f6ad22dd08d4a37215/-/blob/web/src/repo/RepoRevisionContainer.tsx?subtree=true#L127-167) with its multiple calls to `onResolvedRevisionOrError`:

```typescript
// Fetch repository revision.
this.subscriptions.add(
    repoRevisionChanges
        .pipe(
            // Reset resolved revision / error state
            tap(() => this.props.onResolvedRevisionOrError(undefined)),
            switchMap(({ repoName, revision }) =>
                defer(() => resolveRevision({ repoName, revision })).pipe(
                    // On a CloneInProgress error, retry after 1s
                    retryWhen(errors =>
                        errors.pipe(
                            tap(error => {
                                if (isCloneInProgressErrorLike(error)) {
                                    // Display cloning screen to the user and retry
                                    this.props.onResolvedRevisionOrError(error)
                                    return
                                }
                                // Display error to the user and do not retry
                                throw error
                            }),
                            delay(1000)
                        )
                    ),
                    // Save any error in the sate to display to the user
                    catchError(error => {
                        this.props.onResolvedRevisionOrError(error)
                        return []
                    })
                )
            )
        )
        .subscribe(
            resolvedRevision => {
                this.props.onResolvedRevisionOrError(resolvedRevision)
            },
            error => {
                // Should never be reached because errors are caught above
                console.error(error)
            }
        )
)
```

This is easily solved by decoupling data production and side-effects:

```typescript
const resolvedRevisionOrError = useObservable(
    useMemo(
        () => resolveRevision({ repoName: props.repo.name, revision: props.revision}).pipe(
            catchError(error => {
                if (isCloneInProgressErrorLike(error)) {
                    return [error]
                }
                throw error
            }),
            repeatUntil(value => !isCloneInProgressErrorLike(value), { delay: 1000 }),
            catchError(error => [asError(error)])
        ),
        [props.repo.name, props.revision]
    )
)
useEffect(() => {
    props.onResolvedRevisionOrError(resolvedRevisionOrError)
}, [resolvedRevisionOrError, props.onResolvedRevisionOrError])
```
