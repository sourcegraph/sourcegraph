# Pricing

Visit the [pricing page](https://sourcegraph.com/pricing) for up-to-date pricing. 

## How are active users calculated?

Any user who accesses Sourcegraph in a given month is considered a monthly active user (MAU). This includes but is not limited to:

- Conducting a search in the Sourcegraph UI or extension
- Hovering and navigating code in the Sourcegraph UI or extension
- Viewing a file or repository in Sourcegraph
- Creating, viewing or receiving a code monitor
- Creating, viewing, modifying or applying a batch change
- Creating, viewing or modifying a code insight
- Creating, viewing or modifying a search notebook

## How is this measured in the product?

A user who has accessed Sourcegraph is counted as active once they complete an action that represents product usage, such as events logged by the [eventLogger](https://sourcegraph.com/search?q=context:global+repo:sourcegraph/sourcegraph+eventLogger.log%28&patternType=lucky) and events logged by Sourcegraph integrations like browser and IDE extensions, within a specified time period (commonly expressed in daily, weekly or monthly), with [the following filters](https://sourcegraph.com/github.com/sourcegraph/sourcegraph/-/blob/internal/database/event_logs.go?L540):

- All actions done by the system where an event is also logged, such as sending a ping to Sourcegraph. 
- Certain events relating to user authentication ([full list here](https://sourcegraph.com/github.com/sourcegraph/sourcegraph/-/blob/internal/database/event_logs.go?L472)). These same filters are also implemented separetely in our in-product analytics code. 
