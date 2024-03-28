## Сharts

This package provides different visual components primarily for rendering charts and data visualization.
At the moment, this package contains the following list of high-level charts:

- **Series-like line chart** ([storybook](https://storybook.sgdev.org/?path=/story/wildcard-charts--line-charts-vitrina))
  - Grouped
  - Stacked (experimental)
- **Pie chart** ([storybook](https://storybook.sgdev.org/?path=/story/wildcard-charts--pie-chart-vitrina))
- **Categorical-like Bar chart** (experimental) ([storybook](https://storybook.sgdev.org/?path=/story/wildcard-charts--bar-chart-vitrina))
  - Grouped
  - Stacked

## How to use this package

1. Try to use a high-level chart (at the moment, it's either line, bar, or pie chart)
2. If you see that you need minor chart customization (extending existing chart API), feel free to contribute to these charts.
3. If you see that you need an extensive or even mid-size UI customization, you need to use it for a low-level block and build your chart
   in the consumer
4. _As a last resort, you can find similar to your custom chart example in [visx gallery](https://airbnb.io/visx/gallery) and
   bring it to your consumer codebase. Since visx charts use visx primitives, this will not spoil the consistency much.
   If we have more than two consumers needing this new chart, we consider implementing it in the wildcard chart package._

## Basic example

As you can see in the storybook stories of this package, you can use these charts
directly from this package as the following example

```tsx
import { LineChart, LegendList, LegendItem } from './charts'

const SERIES = [
  {
    id: 'series_001',
    data: [
      { x: new Date(10, 10, 2020), value: 94 },
      { x: new Date(11, 10, 2020), value: 134 },
      { x: new Date(12, 10, 2020), value: 134 },
      { x: new Date(13, 10, 2020), value: 123 },
    ],
    name: 'Series 1',
    color: 'var(--blue)',
    getXValue: datum => datum.x,
    getYValue: datum => datum.value,
  },
]

const Example = props => {
  return (
    <div>
      <ParentSize>{parent => <LineChart width={parent.width} height={parent.height} series={SERIES} />}</ParentSize>
      <LegendList>
        {SERIES.map(line => (
          <LegendItem key={line.dataKey.toString()} color={getLineColor(line)} name={line.name} />
        ))}
      </LegendList>
    </div>
  )
}
```

See storybook stories for examples of using other high-level charts.

### High-level charts vs low-lever buildings blocks

Using data visualization components sometimes might be a complex task to do. If high-level components
(components that don't expose too many implementation details) have a lot of props and settings in order
to tune different parts of their UI, even if these props are optional, it complicates the mental model about
these UI components a lot.

In Sourcegraph high-level charts, we're trying to avoid having many surgical settings and props
in their API. Instead, these components should have as small an API surface as possible. If someone in
the product wants to visualize some data, these charts should be the first starting point.

So if you're using (or considering to use) high-level charts components, and **you see that something
that you need to have is missing in chart API, you should think twice before extending high-level chart
API**

We understand that high-level charts, because of minimal surface API, don't cover all possible use cases
for data visualization in the product. And to allow UI chart customization without hurting
high-level charts, we provide **low-level API for building your custom chart** and don't think too much
about implementation details of something that is a standard part of a chart like (axis components, chart
tooltip, etc.)

Our low-level API consists of two major parts

1. **[Visx package primitives](https://airbnb.io/visx)** - visx is an open source library that provides
   small and low-level React wrappers for building your own chart library. These low-level blocks like
   - [@visx/glyph](https://airbnb.io/visx/docs/glyph) - for building complex marks & symbols to be used
     in visuals
   - [@visx/scale](https://airbnb.io/visx/docs/scale) - for mapping data to visual dimensions
   - [@visx/shapa](https://airbnb.io/visx/docs/shape) - a collection of small low-level enhanced svg primitives
   - ... and more. Visx has a lot of small sub packages you can find them in [visx documentation page in chart primitives section](https://airbnb.io/visx/docs).
     We highly-recommend **avoid using high-level chart or block from @visx package**. Based on our experince
     (code insights team) these blocks are useful, but also they might be highly opinionated in some UI parts
     that you would like to change or extend.
2. **This package chart primitives** - sometimes, visx primitives don't include all features we want to
   have for building charts. For example, axis components don't have any label rotation or responsiveness logic.
   We also include Sourcegraph design system styles to get along with other UI of the product. At the moment,  
   we provide a few blocks like that
   - SVG root components - see `./core/SvgRoot.story.tsx`, compound family components for building SVG, chart axis
     and tooltip chart UI (experiment)
   - Smart tooltip component (experiment, it's used in high-level chart but not properly prepared for explicit reusing in
     other consumers)

## Ownership

Even if the code insights team has experience with charts, this doesn't mean we own this charting package.
The primary owner of this package is the frontend platform team (as all components in the wildcard package). Keep this in
mind while you're reading the Roadmap section below.

## Roadmap

_Note: This is the only example of features and improvements that could be made in this package. Please don't take this roadmap
as something that has strict deadlines and owners_

A few big changes that could be made for charts in the future

- **Support different visual types of series in the line char**t. At the moment line chart supports only lines for
  visualization series on the chart. We want to support different visual types for series (for example, grouped or stacked bars).
  At some point, we also want to support different visual types for different series on the chart. So Line chart would become
  SeriesChart.
- **Migrate the Line chart to the new smart axis components**. At the moment, LineChart uses its own version of the axis component.
  These axis component doesn't support label rotation. In the future, we want to support label rotation for all high-level charts in the package.
- **Accessibility improvements**. At the moment bar chart may not work for some screen readers properly.
- **Better low-level API**. At the moment, if you need to create a custom chart, you have to write a lot of custom logic around
  data preparation for the chart, implement custom UI, enforce Sourcegraph styles and support different themes. We should
  expose more low-level API in order to simplify the implementation of custom data visualizations.

Wildcard charts package has [its own GitHub board](https://github.com/orgs/sourcegraph/projects/200/views/44) with all data viz
related issues. If you see something missing, feel free to file an issue with the `data-viz` label.

## Contribution

If you need any help around this package chart or data-visualization in general, feel free to reach out to [code-insights FE team](https://github.com/orgs/sourcegraph/teams/code-insights-frontend)
(@code-insights-fe mention in slack).
Hello World
