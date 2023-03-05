/**
 * A key path that refers to a location in a JSON document.
 *
 * Each successive array element specifies an index in an object or array to descend into. For example, in the
 * object `{"a": ["x", "y"]}`, the key path `["a", 1]` refers to the value `"y"`.
 */
export type KeyPath = (string | number)[]

/**
 * An action contribution describes a command that can be invoked, along with a title, description, icon, etc.
 */
export interface ActionContribution {
    /**
     * The identifier for this action, which must be unique among all contributed actions.
     */
    id: string

    /**
     * The command that this action invokes.
     */
    command?: string

    /**
     * Optional arguments when the action is invoked.
     */
    commandArguments?: (string | number | boolean | null | object | any[])[]

    /** The title that succinctly describes what this action does. The question
     * mark '?' renders the MDI HelpCircleOutline icon. */
    title?: string

    /** The title that succinctly describes what this action is even though it's disabled. */
    disabledTitle?: string

    /**
     * A longer description of the action taken by this action.
     */
    description?: string

    /**
     * A URL to an icon for this action (data: URIs are OK).
     */
    iconURL?: string

    actionItem?: ActionItem
}

/**
 * Narrows the type of {@link ActionContribution} for actions that invoke the `open` client command.
 */
export interface ActionContributionClientCommandOpen extends ActionContribution {
    command: 'open'

    /**
     * The arguments for the `open` client command. The first array element is a URL, which is opened by the client
     * using the default URL handler.
     */
    commandArguments: [string]
}

/**
 * Narrows the type of {@link ActionContribution} for actions that invoke the `updateConfiguration` client command.
 */
export interface ActionContributionClientCommandUpdateConfiguration extends ActionContribution {
    command: 'updateConfiguration'

    /**
     * The arguments for the `updateConfiguration` client command:
     *
     * 1. The key path of the value (in the configuration settings) to update
     * 2. The value to insert at the key path
     * 3. Optional: reserved for future use (must always be `null`)
     * 4. Optional: 'json' if the client should parse the 2nd argument using JSON.parse before inserting the value
     */
    commandArguments: [KeyPath, string | number | boolean | object | any[] | null] | [KeyPath, string, null, 'json']
}

export interface ActionItem {
    /** The text label for this item. */
    label?: string

    /**
     * A description associated with this action item.
     */
    description?: string

    /**
     * The icon URL for this action (data: URIs are OK).
     */
    iconURL?: string

    /**
     * A description of the information represented by the icon.
     */
    iconDescription?: string

    /**
     * Whether the action item should be rendered as a pressed button.
     */
    pressed?: boolean
}
