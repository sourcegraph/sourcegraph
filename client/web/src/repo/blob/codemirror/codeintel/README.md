# Code intel CodeMirror extensions

This document describes the architecture of the code intel CodeMirror
extensions contained in this folder.

The code intel extensions provide the following functionality:

- Token navigation: The ability to navigate from token to token via the
  keyboard
- Code intel tooltips: Information about a specific token (if available)
- Document highlights: Highlight all occurrences of a specific token
- Go to definition: The ability to navigate to the definition for a specific
  token

The main entry point for the whole set of extensions is the
`createCodeIntelExtension`. This functions accepts various configuration
options which allows the extension to interface with the outside world. This
includes communication with the code intel API for requesting the actual code
intel data, and with the rest of the application (e.g. to open the reference
panel).

The extensions themselves can be organized into two groups:

1. Extensions that react to user events or outside input. For example the
   _hover_ or _pin_ extensions.
2. Extensions that receive input from the first group to fetch data from the
   code intel API and update the "enrich" the file view e.g. via decorations.

Each extension is implemented to be relatively self contained. The
communication between extensions mostly happens via [facets][1].

## Extensions reacting to user events

- `gotToDefinitionOnClick` adds mouse event handlers to trigger "go to
  definition" when the user performs a "long click" or "ctrl/cmd click".
- `pinnedLocation` is a facet provided by the file view to specify the
  currently pinned location from the URL. Internally this location is converted
  to a CodeMirror document offset which is used as input to `pinnedRange`.
- `selectedTokenExtension` is a set of extensions that provide token
  navigation. It registers keyboard event handlers to for moving to the
  next/previous token, to trigger "go to definition" for the selected token as
  well has showing/hiding the code intel tooltip for the selected token. It
  provides input for `showTooltip`, `showDocumentHighlights` and
  `ignoreDecoration`.
- `hoverManager` determines whether or not to show a code intel tooltip at the
  cursor position. It provides input for `showTooltip` and `showHasDefinition`
  (to underline the token underneath the cursor if it has a definition
  available).

## "Internal" extensions

- `pinnedRange` holds the document range that's supposed to show a pinned
  tooltip. It fetches tooltip information from the code intel API and
  provides input for `showTooltip`. It calls the `onUnpin` option when the
  selection moves away from this range.
- `showTooltip` holds a list of tooltips (tooltip sources). It waits until
  tooltips are loaded/available and ensures that if multiple tooltips are
  registered for the same range, only one of them is shown. The priority of
  tooltips is determined by the priority of the extensions providing input to
  this facet. It also provides input for `codeIntelDecoration` to highlight the
  token associated with the tooltip.
- `showDocumentHighlights` holds a list of ranges to show document highlights
  for. It fetches document highlight information from the code intel API and
  provides input to `codeIntelDecoration`.
- `showHasDefinition` holds ranges that should be decorated if they have a code
  intel definition associated with it. It requests definitions for the ranges
  from the code intel API and provides input for `codeIntelDecoration`.
- `codeIntelDecoration` and `ignoreDecoration` work to together to ensure that
  token navigation works properly. Basically `codeIntelDecoration` is a then
  wrapper around CodeMirror's own `EditorView.decoration` facet, which filters
  out all decorations that are positioned at `ingoreDecoration`. This is
  necessary to prevent CodeMirror from recreating decorations at the position
  of the selected token, which would cause the editor to loose focus (and
  thus not be able to handle keyboard events for token navigation).

## Code intel API adapter

The `CodeIntelAPIAdapter` class is the only way for extension to communicate
with the code intel API. It's responsible for converting CodeMirror
document ranges/positions to SCIP occurrences, and fetching and converting the
requested data. It's also responsible for avoiding unnecessary
refetching/recomputation. This makes the extensions themselves simpler and
decouples them from code intel specific or app specific logic.

## Token navigation focus problem

We move focus to the currently selected token to provide a better experience
with voice over. The DOM node for indicating the selected token is not stable
however. CodeMirror might recreated DOM nodes when decorations change or remove
them when they move out of the viewport. This can lead to the whole editor
loosing focus and interrupting token navigation. Additional measures are taken
to ensure that the focus stays within the editor.

[1]: https://codemirror.net/docs/guide/#facets
Hello World
