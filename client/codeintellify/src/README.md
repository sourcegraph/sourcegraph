# CodeIntellify

This library manages all of the inputs (mouse/keyboard events, location changes, hover information, and hover actions) necessary to display hover tooltips on with a code view. All together, this makes it easier to add code intelligence to code views on the web. Used in [Sourcegraph](https://sourcegraph.com).

## What it does

- Listens to hover and click events on the code view
- On mouse hovers, determines the line+column position, performs a hover request, and renders it in a nice tooltip overlay at the token
- Shows actions in the hover
- When clicking a token, pins the tooltip to that token
- Highlights the hovered token

You need to provide your own UI component (referred to as the HoverOverlay) that actually displays this information and exposes these actions to the user.

## Usage

- Call `createHoverifier()` to create a `Hoverifier` object (there should only be one on the page, to have only one HoverOverlay shown).
- The Hoverifier exposes an Observable `hoverStateUpdates` that a consumer can subscribe to, which emits all data needed to render the HoverOverlay
- For each code view on the page, call `hoverifier.hoverify()`, passing the position events coming from `findPositionsFromEvents()`.
- `hoverify()` returns a `Subscription` that will "unhoverify" the code view again if unsubscribed from

## Glossary

| Term                | Definition                                                                                                                                                                                                    |
| ------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Code view           | The DOM element that contains all the line elements                                                                                                                                                           |
| Line number element | The DOM element that contains the line number label for that line                                                                                                                                             |
| Code element        | The DOM element that contains the code for one line                                                                                                                                                           |
| Diff part           | The part of the diff, either base, head or both (if the line didn't change). Each line belongs to one diff part, and therefor to a different commit ID and potentially different file path.                   |
| Hover overlay       | Also called tooltip                                                                                                                                                                                           |
| hoverify            | To attach all the listeners needed to a code view so that it will display overlay on hovers and clicks.                                                                                                       |
| unhoverify          | To unsubscribe from the Subscription returned by `hoverifier.hoverify()`. Removes all event listeners from the code view again and hides the hover overlay if it was triggered by the unhoverified code view. |
Hello World
