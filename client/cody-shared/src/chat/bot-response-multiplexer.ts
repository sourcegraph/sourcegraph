/**
 * Processes the part of a response from Cody addressed to a specific topic.
 */
export interface BotResponseSubscriber {
    /**
     * Processes incremental content from the bot. This may be called multiple times during a turn.
     * @param content the incremental text from the bot that was addressed to the subscriber
     */
    onResponse(content: string): Promise<void>

    /**
     * Notifies the subscriber that a turn has completed.
     */
    onTurnComplete(): Promise<void>
}

/**
 * A bot response subscriber that provides the entire bot response in one shot without
 * surfacing incremental updates.
 */
export class BufferedBotResponseSubscriber implements BotResponseSubscriber {
    private buffer_: string[] = []

    /**
     * Creates a BufferedBotResponseSubscriber. `callback` is called once per
     * turn with the bot's entire output provided in one shot. If the topic
     * was not mentioned, `callback` is called with `undefined` signifying the
     * end of a turn.
     * @param callback the callback to handle content from the bot, if any.
     */
    constructor(private callback: (content: string | undefined) => Promise<void>) {}

    // BotResponseSubscriber implementation

    public onResponse(content: string): Promise<void> {
        this.buffer_.push(content)
        return Promise.resolve()
    }

    public async onTurnComplete(): Promise<void> {
        await this.callback(this.buffer_.length ? this.buffer_.join('') : undefined)
        this.buffer_ = []
    }
}

/**
 * Splits a string in one or two places.
 *
 * For example, `splitAt('banana!', 2) => ['ba', 'nana!']`
 * but `splitAt('banana!', 2, 4) => ['ba', 'na!']`
 * @param str the string to split.
 * @param startIndex the index to break the left substring from the rest.
 * @param endIndex the index to break the right substring from the rest, for
 * skipping the middle of the `str` from `[startIndex..endIndex)`.
 * @returns an array with the two substring pieces.
 */
function splitAt(str: string, startIndex: number, endIndex?: number): [string, string] {
    return [str.slice(0, startIndex), str.slice(endIndex === undefined ? startIndex : endIndex)]
}

/**
 * Extracts the tag name from something that looks like a simple XML tag. This is
 * how BotResponseMultiplexer informs the LLM to address specific topics.
 * @param tag the tag, including angle brackets, to extract the topic name from.
 * @returns the topic name.
 */
function topicName(tag: string): string {
    // TODO(dpc): Consider allowing the LLM to put junk in tags like attributes, space, etc.
    const match = tag.match(/^<\/?([A-Za-z-]+)>$/)
    if (!match) {
        throw new Error(`topic tag "${tag}" is malformed`)
    }
    return match[1]
}

/**
 * Incrementally consumes a response from the bot, breaking out parts addressing
 * different topics and forwarding those parts to a registered subscriber, if any.
 */
export class BotResponseMultiplexer {
    /**
     * The default topic. Messages without a specific topic are sent to the default
     * topic subscriber, if any.
     */
    public static readonly DEFAULT_TOPIC = 'Assistant'

    // Matches topic open or close tags
    private static readonly TOPIC_RE = /<$|<\/?([A-Za-z-]?$|[A-Za-z-]+>?)/m

    private subs_ = new Map<string, BotResponseSubscriber>()

    // The topic currently being addressed by the bot. A stack.
    private topics_: string[] = []

    // Gets the topic on the top of the topic stack.
    private get currentTopic(): string {
        return this.topics_.at(-1) || BotResponseMultiplexer.DEFAULT_TOPIC
    }

    // Buffers responses until topics can be parsed
    private buffer_ = ''
    private publishInProgress_ = Promise.resolve()

    /**
     * Subscribes to a topic in the bot response. Each topic can have only one subscriber at a time. New subscribers overwrite old ones.
     * @param topic the string prefix to subscribe to.
     * @param subscriber the handler for the content produced by the bot.
     */
    public sub(topic: string, subscriber: BotResponseSubscriber): void {
        // This test needs to be kept in sync with `TOPIC_RE`
        if (!/^[A-Za-z-]+$/.test(topic)) {
            throw new Error(`topics must be A-Za-z-, was "${topic}`)
        }
        this.subs_.set(topic, subscriber)
    }

    /**
     * Notifies all subscribers that the bot response is complete.
     */
    public async notifyTurnComplete(): Promise<void> {
        // Ensure any existing publishing is done.
        await this.publishInProgress_

        // Flush buffered content, if any
        if (this.buffer_) {
            const content = this.buffer_
            this.buffer_ = ''
            await this.publishInTopic(this.currentTopic, content)
        }

        // Reset to the default topic, ready for another turn
        this.topics_ = []

        // Let subscribers react to the end of the turn.
        await Promise.all([...this.subs_.values()].map(subscriber => subscriber.onTurnComplete()))
    }

    /**
     * Parses part of a compound response from the bot and forwards as much as possible to
     * subscribers.
     * @param response the text of the next incremental response from the bot.
     */
    public publish(response: string): Promise<void> {
        // If an existing publication hasn't finished, convoy behind that one.
        return (this.publishInProgress_ = this.publishInProgress_.then(() => this.publishStep(response)))
    }

    // This is basically a loose parser of an XML-like language which forwards
    // incremental content to subscribers which handle specific tags. The parser
    // is forgiving if tags are not closed in the right order.
    private async publishStep(response: string): Promise<void> {
        this.buffer_ += response
        let last
        while (this.buffer_) {
            if (last !== undefined && last === this.buffer_.length) {
                throw new Error(`did not make progress parsing: ${this.buffer_}`)
            }
            last = this.buffer_.length
            // Look for something that could be a topic.
            const match = this.buffer_.match(BotResponseMultiplexer.TOPIC_RE)
            if (!match) {
                // No topic change is forming, so publish the in-progress content to the current topic
                await this.publishBufferUpTo(this.buffer_.length)
                return
            }
            if (match.index === undefined) {
                throw new TypeError('unreachable')
            }
            if (match.index) {
                // Flush the content before the start (end) topic tag
                await this.publishBufferUpTo(match.index)
                continue // spin again to get a match with resynced indices
            }
            const matchEnd = match.index + match[0].length
            const tagIsOpenTag = match[0].length >= 2 && match[0].at(1) !== '/'
            const tagIsComplete = match[0].at(-1) === '>'
            if (!tagIsComplete) {
                if (matchEnd === this.buffer_.length) {
                    // We must wait for more content to see how this plays out.
                    return
                }
                // The tag is incomplete, but there's content after it, for
                // example: "<--insert coin", match will be "<--insert". Treat
                // it as content.
                await this.publishBufferUpTo(matchEnd)
                continue
            }
            // The tag is complete.
            const topic = topicName(match[0])
            if (!this.subs_.has(topic)) {
                // There are no subscribers for this topic, so treat it as content.
                await this.publishBufferUpTo(matchEnd)
                continue
            }
            this.buffer_ = this.buffer_.slice(matchEnd) // Consume the close tag
            if (tagIsOpenTag) {
                // Handle a new topic
                this.topics_.push(topic)
            } else {
                // Handle the end of a topic: Pop the topic stack until we find a match.
                while (this.topics_.length) {
                    if (this.topics_.pop() === topic) {
                        break
                    }
                }
            }
        }
    }

    // Publishes the content of `buffer_` up to `index` in the current topic. Discards the published content.
    private publishBufferUpTo(index: number): Promise<void> {
        const [content, remaining] = splitAt(this.buffer_, index)
        this.buffer_ = remaining
        return this.publishInTopic(this.currentTopic, content)
    }

    // Publishes one specific topic to its subscriber, if any.
    private async publishInTopic(topic: string, content: string): Promise<void> {
        const sub = this.subs_.get(topic)
        if (!sub) {
            return
        }
        return sub.onResponse(content)
    }

    /** Produces a prompt to describe the response format to the bot. */
    public prompt(): string {
        return `Enclose each part of the response in one of the relevant tags: ${[...this.subs_.keys()]
            .map(topic => `<${topic}>`)
            .join(', ')}:\n\n`
    }
}
