<script lang="ts" context="module">
    function formatDate(date: Date, options: { utc?: boolean }): string {
        if (options.utc) {
            date = addMinutes(date, date.getTimezoneOffset())
        }
        const dateHasTime = date.toString().includes('T')
        const defaultFormat = dateHasTime ? TimestampFormat.FULL_DATE_TIME : TimestampFormat.FULL_DATE
        return format(date, defaultFormat) + (options.utc ? ' UTC' : '')
    }

    enum TimestampFormat {
        FULL_TIME = 'HH:mm:ss',
        FULL_DATE = 'yyyy-MM-dd',
        FULL_DATE_TIME = 'yyyy-MM-dd pp',
    }
</script>

<script lang="ts">
    import { addMinutes, format, formatDistance, formatDistanceStrict } from 'date-fns'
    import { currentDate } from './stores'
    import Tooltip from './Tooltip.svelte'

    /** The date (if string, in ISO 8601 format). */
    export let date: Date | string

    /** Use exact timestamps (i.e. omit "less than", "about", etc.) */
    export let strict: boolean = false

    /** Show absolute timestamp and show relative timestamp in label*/
    export let showAbsolute: boolean = false

    /** Hide suffix (e.g. ago) */
    export let hideSuffix: boolean = false

    /** Show time in UTC */
    export let utc: boolean | undefined = undefined

    $: dateObj = typeof date === 'string' ? new Date(date) : date
    $: formattedDate = formatDate(dateObj, { utc })
    $: relativeDate = (strict ? formatDistanceStrict : formatDistance)(dateObj, $currentDate, {
        addSuffix: !hideSuffix,
    })
</script>

<Tooltip tooltip={showAbsolute ? relativeDate : formattedDate}>
    <span class="timestamp" data-testid="timestamp">{showAbsolute ? formattedDate : relativeDate} </span></Tooltip
>
