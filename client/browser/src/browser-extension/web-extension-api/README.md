# Browser APIs

This directory contains a set of helper functions that wrap the [APIs](https://developer.mozilla.org/en-US/docs/Mozilla/Add-ons/WebExtensions/API) exposed by the browser to browser extensions. This originally came about while implementing support for Safari. Rather than write an entirely new extension, we wrapped the APIs and implemented pseudo support for these APIs in Safari. At this time, we no longer support Safari, however we have kept these because there are still differences in the APIs between Chrome and Firefox. This abstraction also allowed us to have a more type safe interaction with the APIs in actual application code.
Hello World
