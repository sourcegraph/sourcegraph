import path from 'path'
import escapeHTML from 'escape-html'
import * as vscode from 'vscode'
import { readFileSync } from 'fs'

import { Message } from '@sourcegraph/cody-common'

import { EmbeddingsClient } from '../embeddings-client'
import { WSChatClient } from './ws'
import { Prompt } from './prompt'
import { renderMarkdown } from './markdown'
import { getRecipeDisplayText, getRecipeInput } from './recipes'

export interface ChatMessage extends Omit<Message, 'text'> {
	displayText: string
	timestamp: string
}

// If the bot message ends with some prefix of the `Human:` stop
// sequence, trim if from the end.
const STOP_SEQUENCE_REGEXP = /(H|Hu|Hum|Huma|Human|Human:)$/

export class ChatViewProvider implements vscode.WebviewViewProvider {
	private readonly staticDir = ['out', 'static', 'chat']
	private readonly staticFiles = {
		css: ['tabs.css', 'style.css', 'highlight.css'],
		js: ['tabs.js', 'index.js'],
	}

	private transcript: ChatMessage[] = []
	private messageInProgress: ChatMessage | null = null
	private closeConnectionInProgressPromise: Promise<() => void> | null = null
	private prompt: Prompt
	private webview?: vscode.Webview

	constructor(
		private extensionPath: string,
		private wsclient: WSChatClient,
		private embeddingsClient: EmbeddingsClient
	) {
		this.prompt = new Prompt(this.embeddingsClient)
	}

	resolveWebviewView(
		webviewView: vscode.WebviewView,
		_context: vscode.WebviewViewResolveContext<unknown>,
		_token: vscode.CancellationToken
	): void | Thenable<void> {
		this.webview = webviewView.webview
		webviewView.webview.html = this.renderView(webviewView.webview)
		webviewView.webview.options = {
			enableScripts: true,
			localResourceRoots: [vscode.Uri.file(path.join(this.extensionPath, ...this.staticDir))],
		}

		webviewView.webview.onDidReceiveMessage(message => this.onDidReceiveMessage(message, webviewView.webview))
	}

	private async onDidReceiveMessage(message: any, webview: vscode.Webview): Promise<void> {
		switch (message.command) {
			case 'initialized':
				this.sendTranscript()
				break
			case 'reset':
				this.onResetChat()
				break
			case 'submit':
				this.onHumanMessageSubmitted(message.text)
				break
			case 'executeRecipe':
				this.executeRecipe(message.recipe)
				break
		}
	}

	private async sendPrompt(promptMessages: Message[]): Promise<void> {
		await this.closeConnectionInProgress()

		this.closeConnectionInProgressPromise = this.wsclient.chat(promptMessages, {
			onChange: text => this.onBotMessageChange(this.reformatBotMessage(text)),
			onComplete: text => this.onBotMessageComplete(this.reformatBotMessage(text)),
			onError: err => {
				vscode.window.showErrorMessage(err)
			},
		})
	}

	private async closeConnectionInProgress(): Promise<void> {
		if (!this.closeConnectionInProgressPromise) {
			return
		}
		const closeConnection = await this.closeConnectionInProgressPromise
		closeConnection()
		this.closeConnectionInProgressPromise = null
	}

	private async onResetChat(): Promise<void> {
		await this.closeConnectionInProgress()
		this.messageInProgress = null
		this.transcript = []
		this.prompt.reset()
		this.sendTranscript()
	}

	private onNewMessageSubmitted(text: string): void {
		this.messageInProgress = {
			speaker: 'bot',
			displayText: '',
			timestamp: getShortTimestamp(),
		}

		this.transcript.push({
			speaker: 'you',
			displayText: escapeHTML(text),
			timestamp: getShortTimestamp(),
		})

		this.sendTranscript()
	}

	private async onHumanMessageSubmitted(text: string): Promise<void> {
		if (this.messageInProgress) {
			return
		}
		this.onNewMessageSubmitted(text)

		const promptMessages = await this.prompt.constructPromptForHumanInput(text)
		return this.sendPrompt(promptMessages)
	}

	async executeRecipe(recipe: string): Promise<void> {
		if (this.messageInProgress) {
			vscode.window.showErrorMessage(
				'Cannot execute multiple recipes. Please wait for the current recipe to finish.'
			)
			return
		}

		const input = getRecipeInput(recipe)
		if (!input) {
			return
		}
		this.showTab('chat')

		const displayText = getRecipeDisplayText(recipe, input)
		this.onNewMessageSubmitted(displayText)

		const promptMessages = await this.prompt.constructPromptForRecipe(recipe, input)
		return this.sendPrompt(promptMessages)
	}

	private reformatBotMessage(text: string): string {
		let reformattedMessage = text.trimEnd()

		const stopSequenceMatch = reformattedMessage.match(STOP_SEQUENCE_REGEXP)
		if (stopSequenceMatch) {
			reformattedMessage = reformattedMessage.slice(0, stopSequenceMatch.index)
		}
		// TODO: Detect if bot sent unformatted code without a markdown block.
		return fixOpenMarkdownCodeBlock(reformattedMessage)
	}

	private onBotMessageChange(text: string): void {
		this.messageInProgress = {
			speaker: 'bot',
			displayText: renderMarkdown(text),
			timestamp: getShortTimestamp(),
		}

		this.sendTranscript()
	}

	private onBotMessageComplete(text: string): void {
		this.messageInProgress = null
		this.closeConnectionInProgressPromise = null
		this.transcript.push({
			speaker: 'bot',
			displayText: renderMarkdown(text),
			timestamp: getShortTimestamp(),
		})

		this.prompt.addBotResponse(text)

		this.sendTranscript()
	}

	private showTab(tab: string): void {
		this.webview?.postMessage({ type: 'showTab', tab })
	}

	private sendTranscript() {
		this.webview?.postMessage({
			type: 'transcript',
			messages: this.transcript,
			messageInProgress: this.messageInProgress,
		})
	}

	renderView(webview: vscode.Webview): string {
		const html = readFileSync(path.join(this.extensionPath, ...this.staticDir, 'index.html')).toString()

		const nonce = getNonce()
		return html
			.replace('{nonce}', nonce)
			.replace('{scripts}', this.staticFiles.js.map(file => this.getScriptTag(webview, file, nonce)).join(''))
			.replace('{styles}', this.staticFiles.css.map(file => this.getStyleTag(webview, file)).join(''))
	}

	private getScriptTag(webview: vscode.Webview, filePath: string, nonce: string): string {
		const src = webview.asWebviewUri(vscode.Uri.file(path.join(this.extensionPath, ...this.staticDir, filePath)))
		return `<script nonce="${nonce}" src="${src}"></script>`
	}

	private getStyleTag(webview: vscode.Webview, filePath: string): string {
		const href = webview.asWebviewUri(vscode.Uri.file(path.join(this.extensionPath, ...this.staticDir, filePath)))
		return `<link rel="stylesheet" href="${href}">`
	}
}

function fixOpenMarkdownCodeBlock(text: string): string {
	const occurances = text.split('```').length - 1
	if (occurances % 2 === 1) {
		return text + '\n```'
	}
	return text
}

function padTimePart(timePart: number): string {
	return timePart < 10 ? `0${timePart}` : timePart.toString()
}

function getShortTimestamp() {
	const date = new Date()
	return `${padTimePart(date.getHours())}:${padTimePart(date.getMinutes())}`
}

function getNonce() {
	let text = ''
	const possible = 'ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789'
	for (let i = 0; i < 32; i++) {
		text += possible.charAt(Math.floor(Math.random() * possible.length))
	}
	return text
}
