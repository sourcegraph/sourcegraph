# Wildcard combobox primitives

These components are simply wrappers over the [reach-ui combobox primitives](https://reach.tech/combobox#comboboxpopover).
We use these reach-ui primitives to have covered all a11 peculiarities in how combobox UI should work.

For more details, we suggest taking a look at `Combobox.story.tsx` file ([storybook demos](https://storybook.sgdev.org/?path=/story/wildcard-combobox--combobox-demo)) but in a nutshell Combobox
the package exposes low-level UI blocks that

- Enforce Sourcegraph-like styles
- Enforce all-important accessibility aspects around keyboard and screen-reader navigations.

```tsx
import { Combobox, ComboboxInput, ComboboxPopover, ComboboxList, ComboboxOption } from '@sourcegraph/wildcard'

function RepositoriesCombobox() {
  return (
    <Combobox aria-label="Choose a repo" style={{ maxWidth: '20rem' }}>
      <ComboboxInput
        label="Repository"
        placeholder="Start type..."
        message="You need to specify repo name (github.com/sg/sg) and then pick one of the suggestions items."
      />

      <ComboboxPopover>
        <ComboboxList>
          <ComboboxOption value="github.com/sourcegraph/sourcegraph" />
          <ComboboxOption value="github.com/sourcegraph/about" />
          <ComboboxOption value="github.com/sourcegraph/deploy" />
          <ComboboxOption value="github.com/sourcegraph/handbook" />
        </ComboboxList>
      </ComboboxPopover>
    </Combobox>
  )
}
```

Where

- `<Combobox />` is the main root component for all other Combobox compound components. It stores all important
  information about the combobox state (without this component, you can't use Combobox UI)
- `<ComboboxInput />` is an input component, you can override component implementation with `as` prop, but by
  default, we use a standard text `<Input />` component from the wildcard package.
- `<ComboboxPopover />` (\*option if you don't want to render suggestions in a popover). It adds a popover component where
  all suggestions will be rendered, by default, it uses the standard `Popover` component from the wildcard.
- `<ComboboxList />` is a compound wrapper for all suggestions.
- `<ComboboxOption />` is a suggestions text component (it implements text highlighting logic by default)

[See Combobox storybook stories for more examples](https://storybook.sgdev.org/?path=/story/wildcard-combobox--combobox-demo).
Hello World
