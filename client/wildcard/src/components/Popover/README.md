## Popover components

This package provides special React components that allow you to build popovers, tooltips, or floating panels.

### Standard use case

By default `<Popover />` component requires no props if you're using compound Popover components.

```tsx
import { Popover, PopoverTrigger, PopoverContent } from '@sourcegraph/wildcard'

function ComponentWithPopover() {
  return (
    <Popover>
      <PopoverTrigger>Click me</PopoverTrigger>

      <PopoverContent>Any popover content here</PopoverContent>
    </Popover>
  )
}
```

This demo renders a standard button component with a connected popover/popup element. By clicking on the `PopoverTrigger` button,
you can see `PopoverContent`, which is placed based on the `PopoverTrigger` position, document, and other scroll elements
scroll positions and available space of page viewport. By click outside of popover element or by the second click on the `PopoverTrigger` element
PopoverContent will be hidden and unmounted from the DOM.

See _**Standard storybook example**_ of this component in `./Popover.story.tsx` for more details.

### Controlled component mode

By default, the `PopoverTrigger` component only listens to clicks to trigger the `PopoverContent` appearance. Still, you may
want to take control over this and render the popup by your own logic. For example, closing Popover when a form in
`PopoverContent` has been submitted, or any other async operation is over.

```tsx
import { Popover, PopoverTrigger, PopoverContent } from '@sourcegraph/wildcard'

function ComponentWithPopover() {
  const [open, setOpen] = useState<boolean>(false)
  const handleSubmit = async function (event) {
    const form = event.target
    const formValues = new FormData(form)

    /*
     * Any async or sync valid operations
     */

    // Closing the popover form
    setOpen(false)
  }

  return (
    <Popover open={open} onOpenChange={event => setOpen(event.isOpen)}>
      <PopoverTrigger>Click me</PopoverTrigger>

      <PopoverContent>
        <form onSubmit={handleSubmit}>{/* Input and other content*/}</form>
      </PopoverContent>
    </Popover>
  )
}
```

In the example above, we are still listening to all popover open state-changing events (trigger element clicks, keyboard, outside clicks)
by `onOpenChange` callback, but the consumer now is in charge of carrying popover open state.

Another popular case - render tooltip/popover when the target got focused. An accessible version of the code intel tooltip might be a good example.
For instance, when some hoverable literal element got focused in the blob view, we want to do a few things

- Render the code intel tooltip right next to the target
- Move focus from the target into the tooltip and focus first focusable element within the tooltip
- By clicking outside or ESC move focus back to the target and close the tooltip (in order not to break UX of focus navigation)

So the tricky part here is the third step, we need to focus on the target after the tooltip gets closed, but
we should not open the tooltip again because we close it. To achieve this, we will use a controlled mode of Popover
components and FSM (finite state machine) pattern to avoid looping show/hide logic for popover appearance.

```tsx
enum FSM_STATES {
  Initial = 'Initial',
  PopupOpened = 'PopupOpened',
  FocusedAfterPopupClosed = 'FocusedAfterPopupClosed',
}

enum FSM_ACTIONS {
  TargetFocus,
  TargetBlur,
  PopupClose,
}

const FSM_TRANSITIONS: Record<FSM_STATES, Partial<Record<FSM_ACTIONS, FSM_STATES>>> = {
  [FSM_STATES.Initial]: {
    [FSM_ACTIONS.TargetFocus]: FSM_STATES.PopupOpened,
  },
  [FSM_STATES.PopupOpened]: {
    [FSM_ACTIONS.PopupClose]: FSM_STATES.FocusedAfterPopupClosed,
  },
  [FSM_STATES.FocusedAfterPopupClosed]: {
    [FSM_ACTIONS.TargetBlur]: FSM_STATES.Initial,
  },
}

const ShowOnFocus = () => {
  const [state, setState] = useState<FSM_STATES>(FSM_STATES.Initial)

  const handleOpenChange = (event: Popover.PopoverOpenEvent): void => {
    if (!event.isOpen) {
      setState(FSM_TRANSITIONS[state][FSM_ACTIONS.PopupClose])
    }
  }

  const handleTargetFocus = () => {
    setState(FSM_TRANSITIONS[state][FSM_ACTIONS.TargetFocus])
  }

  const handleTargetBlur = () => {
    setState(FSM_TRANSITIONS[state][FSM_ACTIONS.TargetBlur])
  }

  const open = state === FSM_STATES.PopupOpened

  return (
    <Popover.Root open={open} onOpenChange={handleOpenChange}>
      <Popover.Trigger as={Button} onFocus={handleTargetFocus} onBlur={handleTargetBlur}>
        Target
      </Popover.Trigger>

      <Popover.Content>Popover content</Popover.Content>
    </Popover.Root>
  )
}
```

See `ShowOnFocus` storybook story to try it in action.

### Components overview

Let's see the popover collection components and their props/settings in detail.

### `Popover`

This is the root component that renders nothing but its children.

- **_anchor_** (optional) - React ref of the element by trigger on we should show/hide popover element
- **_open_** (optional) - Controlled state that is responsible for showing/hiding the `PopoverContent` component
- **_onOpenChange_** (optional) - it's called whenever open state supposed to be changed (trigger target action, Esc button, click outside the target)

### `PopoverTrigger`

It doesn't have any specific Popover props. By default, the renders button hence has all buttons native props.

### `PopoverContent`

Renders div element with children outside of main DOM hierarchy (it would be the last tag of document body tag)

- **_open_** (optional) - Controlled state that is responsible for showing/hiding the `PopoverContent`. It has more priority
  than the `open` prop of the root `Popover` component.
  **_focusLocked_** (optional) - Locks focus inside the popover element and focus the first focusable elements within the popover content.
- **_pin_** (optional) - If it's passed, the target ref will be ignored, and position will be calculated around this point.
  It could be a mouse cursor or some coordinate on a canvas chart.
- **_position_** (optional) - Initial tooltip position. Tooltip position calculator can change it
  during position calculation that takes into account layout data (constraints,
  viewport space, paddings, etc.)

```ts
enum Position {
  topStart,
  top,
  topEnd,
  leftStart,
  left,
  leftEnd,
  rightStart,
  right,
  rightEnd,
  bottomStart,
  bottom,
  bottomEnd,
}
```

- **_flipping_** (optional) - It specifies the flipping position strategy when the current position's (`position` prop) side doesn't have enough space to fit the popover element.

```ts
enum Flipping {
  /**
   * Whenever tooltip doesn't have enough space then pick any other position
   * based on initial position that tooltip has.
   *
   * Example: left → right → bottom → top
   */
  all = 'all',

  /**
   * Whenever tooltip doesn't have enough space then pick only opposite position
   * of whatever initial position tooltip got.
   *
   * Example: left → right only
   * Example: top → bottom only
   */
  opposite = 'opposite',
}
```

- **_overlapping_** (optional) - Allows tooltip to overlap target element if it's
  needed (not enough space with current position)
- **_overflowToScrollParents_** (optional) - If it's true then it hides tooltip when target isn't visible due scroll containers
- **_constrainToScrollParents_** (optional) - If it's true then it fits popover element' position and sizes into target scroll containers.
- **_strategy_** (optional) - Setups position strategy (Fixed or Absolute) to render the popover element.

```ts
enum Strategy {
  /**
   * Fixed strategy renders popover element outside of DOM hierarchy in the designated
   * container in the body element
   */
  Fixed,

  /**
   * Absolute strategy renders popover element next to the target element and
   * calculate its position based on the nearest container with position relative.
   */
  Absolute,
}
```

- **_targetPadding_** (optional) - Adds space/padding between target and popover elements
Hello World
