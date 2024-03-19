# How to use a screen reader

Screen readers are tools that are used by users who are blind or have limited vision to navigate and describe webpages. It is important that we understand how to correctly use them when building our application.

Depending on the platform, there are two widely used screen readers. Although they provide different controls, they ultimately parse webpages in a similar manner. You should not feel that you need to test a page using both screen readers, one is sufficient.

## VoiceOver (MacOS)

<i>VoiceOver is already available on MacOS, no download is required.</i>

[Video tutorial](https://youtu.be/5R-6WvAihms)

**Cheatsheet**

| Shortcut   |      Action      |
|------------|:----------------:|
| <kbd>Command</kbd> + <kbd>F5</kbd> | Enable/Disable VoiceOver |
| <kbd>Control</kbd> | Stop talking |
| <kbd>Control</kbd> + <kbd>Option</kbd> + <kbd>Space</kbd> | Click |
| <kbd>Control</kbd> + <kbd>Option</kbd> + <kbd>→</kbd> | Navigate to next line |
| <kbd>Control</kbd> + <kbd>Option</kbd> + <kbd>←</kbd> | Navigate to previous line |
| <kbd>Control</kbd> + <kbd>Option</kbd> + <kbd>Command</kbd> + <kbd>H</kbd> | Navigate to next heading |
| <kbd>Control</kbd> + <kbd>Option</kbd> + <kbd>Command</kbd> + <kbd>Shift</kbd> + <kbd>H</kbd> | Navigate to previous heading |


## NVDA (Windows)

<i>Download at https://www.nvaccess.org/download/</i>

[Video tutorial](https://youtu.be/Jao3s_CwdRU)

**Cheatsheet**

| Shortcut   |      Action      |
|----------|:-------------:|
| <kbd>Control</kbd> + <kbd>Alt</kbd> + <kbd>N</kbd> | Enable/disable NVDA |
| <kbd>Caps Lock</kbd> | Stop talking |
| <kbd>Enter</kbd> | Click |
| <kbd>↓</kbd> | Navigate to next line |
| <kbd>↑</kbd> | Navigate to previous line |
| <kbd>H</kbd> | Navigate to next heading |
| <kbd>Shift</kbd> + <kbd>H</kbd> | Navigate to previous heading |

## Tips

- Use [`aria-hidden`](https://developer.mozilla.org/en-US/docs/Web/Accessibility/ARIA/Attributes/aria-hidden) to hide decorative elements from screen readers.
- Use ReachUI's [`<VisuallyHidden />`](https://reach.tech/visually-hidden/) to provide accessible text that is visually hidden from the UI. This is useful for communicating the semantic meaning of buttons or other elements which are visually represented with an icon or other purely graphical content.
  - Note: `VisuallyHidden` and `sr-only` can sometimes cause visual bugs. See [this related Chromium bug report](https://bugs.chromium.org/p/chromium/issues/detail?id=1154640). We should only use this approach where it is not possible to use an equivalent `aria-` attribute.
- Provide a more specific `aria-label` to elements whose exact meaning requires context around it to understand.
  - For example, in a list of items where each item also has a button associated with it, the button should be `aria-label`ed to more clearly indicate which item the button is for.
- Use `screenReaderAnnounce` to communicate messages about implied changing state (e.g. form submission, async resolutions, or changes to on-screen data from polling).
- Use [landmark roles](https://developer.mozilla.org/en-US/docs/Web/Accessibility/ARIA/Roles/region_role) such as `role="region"` to identify areas with multiple pieces of content that users would want to navigate to easily.
