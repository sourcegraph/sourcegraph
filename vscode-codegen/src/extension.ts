import * as vscode from 'vscode'
import { CompletionsDocumentProvider } from './docprovider'
import { History } from './history'
import { ChatViewProvider } from './chat/view'
import { WSChatClient } from './chat/ws'
import { WSCompletionsClient, fetchAndShowCompletions } from './completions'
import { EmbeddingsClient } from './embeddings-client'

const CODY_ENDPOINT = 'cody.sgdev.org'

export async function activate(context: vscode.ExtensionContext) {
	console.log('Cody extension activated')
	const isDevelopment = process.env.NODE_ENV === 'development'

	const settings = vscode.workspace.getConfiguration()
	const documentProvider = new CompletionsDocumentProvider()
	const history = new History()
	history.register(context)

	const serverAddr = settings.get('cody.serverEndpoint') || CODY_ENDPOINT
	const serverUrl = `${isDevelopment ? 'ws' : 'wss'}://${serverAddr}`

	const embeddingsAddr = settings.get('cody.embeddingsEndpoint') || CODY_ENDPOINT
	const embeddingsUrl = `${isDevelopment ? 'http' : 'https'}://${embeddingsAddr}`

	const codebaseId: string = settings.get('cody.codebase', '')
	if (!codebaseId) {
		vscode.window.showWarningMessage(
			'Cody needs a codebase to work with. Please set the "cody.codebase" setting in your workspace settings and reload the editor.'
		)
	}

	const accessToken = (await context.secrets.get('cody.access-token')) ?? ''
	if (!accessToken) {
		vscode.window.showWarningMessage(
			'Cody needs an access token to work. Please set the token using the "Cody: Set access token" command and reload the editor.'
		)
	}

	const wsCompletionsClient = WSCompletionsClient.new(`${serverUrl}/completions`, accessToken)
	const wsChatClient = WSChatClient.new(`${serverUrl}/chat`, accessToken)
	const embeddingsClient = codebaseId ? new EmbeddingsClient(embeddingsUrl, accessToken, codebaseId) : null

	const chatProvider = new ChatViewProvider(context.extensionPath, wsChatClient, embeddingsClient)

	const executeRecipe = async (recipe: string) => {
		await vscode.commands.executeCommand('cody.chat.focus')
		return chatProvider.executeRecipe(recipe)
	}

	const storeAccessToken = async (accessToken: string | undefined) => {
		if (!accessToken) {
			return
		}
		context.secrets.store('cody.access-token', accessToken)
	}

	context.subscriptions.push(
		vscode.workspace.registerTextDocumentContentProvider('codegen', documentProvider),
		vscode.languages.registerHoverProvider({ scheme: 'codegen' }, documentProvider),

		vscode.commands.registerCommand('cody.suggest', async () => {
			await fetchAndShowCompletions(wsCompletionsClient, documentProvider, history)
		}),

		vscode.commands.registerCommand('cody.recipe.explain-code', async () => executeRecipe('explainCode')),

		vscode.commands.registerCommand('cody.recipe.explain-code-high-level', async () =>
			executeRecipe('explainCodeHighLevel')
		),

		vscode.commands.registerCommand('cody.recipe.generate-unit-test', async () =>
			executeRecipe('generateUnitTest')
		),

		vscode.commands.registerCommand('cody.recipe.generate-docstring', async () =>
			executeRecipe('generateDocstring')
		),

		vscode.window.registerWebviewViewProvider('cody.chat', chatProvider),

		vscode.commands.registerCommand('cody.set-access-token', async () => {
			const tokenInput = await vscode.window.showInputBox()
			await storeAccessToken(tokenInput)
		})
	)
}

export function deactivate() {}
