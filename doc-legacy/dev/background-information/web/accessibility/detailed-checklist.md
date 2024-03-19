# Detailed Checklist

<i>This checklist is written against the [WCAG 2.1](https://www.w3.org/TR/WCAG21/) AA specification. If, in the future, we are required to meet other WCAG versions or grades, we should revise this checklist.</i>

## Prerequisites
- You have the ARC Toolkit browser extension installed.
- You have read and understood [How to use ARC Toolkit](how-to-arc-toolkit.md).

## Checklist

**Note:** Some parts of this checklist may not be relevant to your user journey (e.g. maybe you're not using any tables). That's OK, just skip those parts.

### Headings

<details>
<summary>Click to expand</summary>

**We should ensure that:**

- All headings use the correct semantic `<hX>` tag (e.g. `<h1>`).
- There is only be one `<h1>` element per page.
- All headings are written in the correct logical sequence.
  - The order should descend, a `<h4>` should not appear on the page before the first `<h3>` element
  - Heading levels should not be skipped. Don't jump from `<h1>` to `<h3>` and skip `<h2>`. See the *tips* section below for how you can fix styling issues caused by this.
- All heading text correctly describes the page/section it is in.

**How to test:**

- Run an audit in ARC Toolkit. Select the "Headings" test group. It will visually identify each heading on the page and raise any warnings.
- Navigate through the journey with a screen reader. Use only the "Navigate to next heading" shortcut.

**Tips:**

- If we have to use an `<h2>` but need the styles of an `<h3>`, we can easily cross styles with the [Wildcard `<Heading />` components](https://storybook.sgdev.org/?path=/story/wildcard-typography--crossing-styles). For example:
  - `<H3 as={H2}>Hello</H3>` will _downscale_ a style, rendering an `<h2>` with the styles of an `<h3>`
  - `<Heading as="h2" styleAs="h1">Hello</Heading>` will _upscale_ a style, rendering an `<h2>` with the styles of an `<h1>`
- If you are still unsure about something, consult the [W3 guide on headings](https://www.w3.org/WAI/tutorials/page-structure/headings/).
</details>

### Images / Graphical content

<details>
<summary>Click to expand</summary>

**We should ensure that:**

- All `<img>` elements have an appropriate descriptive attribute
  - If it adds value to the user journey, the `alt` should describe the image.
  - If it does not add value (it is purely decorative), the `alt` attribute should explicitly be set as "".
    - Example: `<img alt=""/>`
  - If it contains text, the alt description should include the image's text.
- All `<svg>` elements have an appropriate descriptive attribute
  - If it adds value to the user journey, it should include a `<title>` element within the SVG. This element should be referenced through `aria-labelledby` on the `svg` element.
    - Example: `<svg aria-labelledby="svgtitle1"><title id="svgtitle1">Settings</title> [other svg code]</svg>`
    - **Note:** Our `mdi-react` icons currently do not support injecting a `<title>` element. We are currently investigating a solution in [this issue](https://github.com/sourcegraph/sourcegraph/issues/32379). 

**How to test:**

- Run an audit in ARC Toolkit. Select the "Images" test group. It will visually identify each image on the page and raise any warnings.
- Navigate through the journey with a screen reader. Watch out for any "unlabelled image" (or similar) readouts.

**Tips:**

- If you are still unsure about something, consult the [W3 guide on images](https://www.w3.org/WAI/tutorials/images/).
</details>

### Lists

<details>
<summary>Click to expand</summary>

**We should ensure that:**

- Lists of items that are related to each other use correct list elements.
  - Documentation on each list element: [`<ol>`](https://developer.mozilla.org/en-US/docs/Web/HTML/Element/ol), [`<ul>`](https://developer.mozilla.org/en-US/docs/Web/HTML/Element/ul), [`<li>`](https://developer.mozilla.org/en-US/docs/Web/HTML/Element/li), [`<dl>`](https://developer.mozilla.org/en-US/docs/Web/HTML/Element/dl).

**How to test:**

- Run an audit in ARC Toolkit. Select the "Lists" test group. It will visually identify each list on the page and raise any warnings. Check that everything that *should* be a list *is* a list.
</details>

### Links

<details>
<summary>Click to expand</summary>

**We should ensure that:**

- All actions that navigate use the `<a>` element.
- We never use the `<a>` element for an action that *does not* navigate.
- All `<a>` elements have a valid `href` attribute that describes where the user will be navigated to.
- All navigation actions are recognizable as links.
- All navigation actions are correct focus styles.

**How to test:**

- Run an audit in ARC Toolkit. Select the "Links" test group. It will visually identify each link on the page and raise any warnings. Check that everything that *should* be a link *is* a link.

**Tips**

- Use the [Wildcard `<Link />` component](https://storybook.sgdev.org/?path=/story/wildcard-link--simple).
</details>

### Buttons

<details>
<summary>Click to expand</summary>

**We should ensure that:**

- All actions that do not navigate use the `<button>` element.
- All `<button>` elements have a valid `type` attribute that describes the action.
- All button elements are recognizable as buttons.
- All button elements have correct focus styles.

**How to test:**

- Run an audit in ARC Toolkit. Select the "Buttons" test group. It will visually identify each button on the page and raise any warnings. Check that everything that *should* be a button *is* a button.

**Tips**

- Use the [Wildcard `<Button />` component](https://storybook.sgdev.org/?path=/story/wildcard-button--simple).
</details>

### Tables

<details>
<summary>Click to expand</summary>

**We should ensure that:**

- All tabular data uses the [`<table>`](https://developer.mozilla.org/en-US/docs/Web/HTML/Element/table) element and associated child elements such as `th` and `td`.
  - Do we need to display data in rows and columns? Use `<table>`.

**How to test:**

- Run an audit in ARC Toolkit. Select the "Tables" test group. It will visually identify each table on the page and raise any warnings.

**Tips**

- If you are still unsure about something, consult the [W3 guide on images](https://www.w3.org/WAI/tutorials/tables/).
</details>

### Forms

<details>
<summary>Click to expand</summary>

**We should ensure that:**

- All form inputs have a corresponding `<label>` element or `aria-label` attribute.
- Related form elements are grouped with `<fieldset>`.
  - Example: A group of radio buttons should be grouped within a `fieldset`
- Any errors are correctly associated with the relevant input.
  - Tip: Use `aria-describedby` to quickly link any relevant labels to the input.
- It is possible to identify error, warning and success states of the form through text.
  - We cannot assume that a user can identify these through color alone.
  - Note: Sometimes certain states can be implied (like seeing your form submission appear on the page). We still need to support users who won't be able to identify this. In this case, we should use `screenReaderAnnounce` to communicate messages to screen readers.

**Note:** Forms can be complex! We heavily encourage you to to seek further information using the [W3 forms guide](https://www.w3.org/WAI/tutorials/forms/). Accessibility guidelines can differ depending on the type of form.

**How to test:**

- Run an audit in ARC Toolkit. Select the "Forms" test group. It will visually identify each form and input on the page, show respective labels and raise any warnings.
- Complete the form using a screen reader. Be sure to test all states, including any errors.

**Tips**:

- If you are still unsure about something, consult the [W3 guide on forms](https://www.w3.org/WAI/tutorials/forms/).
</details>

### Color contrast

<details>
<summary>Click to expand</summary>

**We should ensure that:**

- Borders and icons have a contrast ratio of at least 3:1 against their backgrounds.
- Body text has contrast ratio of at least 4.5:1 against its background.
- Large text has a contrast ratio of at least 3:1 against its background.

**How to test:**

- Run an audit in ARC Toolkit. Select the "Color Contrast" test group. It will report any contrast violations on the current page.
- Be sure to run this audit in **both** dark and light themes.
</details>

