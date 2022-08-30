export enum GroupResultsPing {
    // Aggregation chart events
    ChartBarClick = 'GroupResultsChartBarClick',
    ChartBarHover = 'GroupResultsChartBarHover',

    // Aggregation mode events
    ModeClick = 'GroupAggregationModeClicked',
    ModeDisabledHover = 'GroupAggregationModeDisabledHover',

    // Other UI
    CollapseSidebarSection = 'GroupResultsCollapseSection',
    ExpandSidebarSection = 'GroupResultsOpenSection',
    ExpandFullViewPanel = 'GroupResultsExpandViewOpen',
    CollapseFullViewPanel = 'GroupResultsExpandedViewCollapse',
    InfoIconHover = 'GroupResultsInfoIconHover',
}
