# Bar Chart

The bar chart is one of the pre-built high-level data visualization components. Since it's a pre-built
component it has a few UI features that are already implemented inside the component.

- **Data grouping**. A bar chart can render data in different modes,
  - Plain list of bars (groups)
  - Grouped by categories' data. In this mode, you can group your bars with
    different categories (see Grouped bar chart demo in the bar chart storybook)
  - Stacked bars
- **Smart axis components**. Smart axes adjust label UI based on chart size. Small charts
  rotate their labels in order to avoid visual collisions between labels
- **Smart active/non-active bar colors**. The bar chart has a smart color calculation
  algorithm that produces dimmed colors as you hover one of the bars (it has a limitation though,
  see the section about colors below)

## Colors

By default, the bar chart requires the `getDatumColor` prop that should specify colors for bars (groups)
on the chart. When we hover one of the bars, we should dim all other bars on the chart. For that, we
use a CSS filters algorithm that tries to convert the current color (the one that we got from the `getDatumColor` prop)
and make it dimmed. This algorithm allows you to have proper dimmed colors when you don't have
clear defined colors for your chart (for example, code insight can have any color that the user defines in the creation UI,
and we can't have all color combinations for insights chart, we need to calculate colors dynamically) However, this
algorithm doesn't work well in all cases. Sometimes when you're using bright colors, it may produce low-contrast colors.

To solve this problem, if you need to use bright colors on the chart, and you want to get control
over active/non-active colors, you can specify colors for the bar non-active state manually with `getDatumFadeColor` prop.
If you set this prop, this turns off the generic color algorithm for non-active bars and takes the color provided by you.
Hello World
