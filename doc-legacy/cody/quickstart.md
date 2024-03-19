# Cody Quickstart

<p class="subtitle">In this quickstart guide, you'll learn how to use Cody once you have installed the extension in your VS Code editor. Here you will perform the following three tasks:</p>

1. Generate unit tests for your code
2. Identify errors and get suggested code fixes
3. Add documentation to your code snippets

## Prerequisites

- Make sure you have the [Cody extension installed](overview/install-vscode.md) in your VS Code editor
- You have a Free or Pro account through Sourcegraph.com or a Sourcegraph Enterprise account
- You have a project open in VS Code

## Getting started with the Cody extension and commands

After installing the extension, the side activity bar will display an icon for **Cody**. Click this icon, and Cody's panel will open. This interface is used to start a **New Chat**, run Cody **commands** or get access to relevant resources.

![Cody icon in side activity bar ](https://storage.googleapis.com/sourcegraph-assets/Docs/NLS-Beta.png)

Cody supports **Commands** with VS Code. These are quick, ready-to-use prompt actions that you can apply to any code or text-based snippet you've highlighted. You can run a command in the following ways:

1. Highlight your code and select the command from the sidebar

![running-commands](https://storage.googleapis.com/sourcegraph-assets/Docs/using-commands.png)

2. Open up the command palette with `Option+C` `Alt+C`
3. Right-click on any code element and select **Cody > Choose a command** from the list
4. Type `/` in the chat bar. Cody will then suggest a list of available commands

![slash-command](https://storage.googleapis.com/sourcegraph-assets/Docs/slash-command-trigger.png)

## Working with the Cody extension

Let's create a JavaScript function that converts a `given date` into a human-readable description of the time elapsed between the `given date` and the `current date`. This example uses a starter boilerplate code that helps you set up and run three tasks with the Cody VS Code extension.

Here's the code for the date elapsed function:

```js
function getTimeAgoDescription(dateString) {
	const startDate = new Date(dateString);
	const currentDate = new Date();

	const years = currentDate.getFullYear() - startDate.getFullYear();
	const months = currentDate.getMonth() - startDate.getMonth();
	const days = currentDate.getDate() - startDate.getDate();

	let timeAgoDescription = '';

	if (years > 0) {
		timeAgoDescription += `${years} ${years === 1 ? 'year' : 'years'}`;
	}

	if (months > 0) {
		if (timeAgoDescription !== '') {
			timeAgoDescription += ' ';
		}
		timeAgoDescription += `${months} ${months === 1 ? 'month' : 'months'}`;
	}

	if (days > 0) {
		if (timeAgoDescription !== '') {
			timeAgoDescription += ' ';
		}
		timeAgoDescription += `${days} ${days === 1 ? 'day' : 'days'}`;
	}

	if (timeAgoDescription === '') {
		timeAgoDescription = 'today';
	} else {
		timeAgoDescription += ' ago';
	}

	return timeAgoDescription;
}
```

## 1. Generate a unit test

To ensure code quality and early bug detection, one of the most useful commands that Cody offers is **Generate unit tests**. It quickly helps you write test code for any snippet that you have highlighted. To generate a unit test for our example function:

- Open the `date.js` file in VS Code
- Highlight a code snippet that you'd like to test
- Inside Cody chat, type `/test` or press the command hotkey (`⌥` + `c` / `alt` + `c`)
- Select `Generate unit tests` option and hit `Enter`

Cody will help you generate the following unit test in the sidebar:

```js
import assert from 'assert';
import { getTimeAgoDescription } from '../src/date.js';

describe('getTimeAgoDescription', () => {
	it('returns correct relative time for date 1 month ago', () => {
		const oneMonthAgo = new Date(Date.now() - 30 * 24 * 60 * 60 * 1000).toISOString().split('T')[0];
		const result = getTimeAgoDescription(oneMonthAgo);
		assert.equal(result, '1 month ago');
	});

	it('returns correct relative time for date 2 months ago', () => {
		const twoMonthsAgo = new Date(Date.now() - 2 * 30 * 24 * 60 * 60 * 1000).toISOString().split('T')[0];
		const result = getTimeAgoDescription(twoMonthsAgo);
		assert.equal(result, '2 months ago');
	});
});

```

This unit test file tests the `getTimeAgoDescription()` function from the `date.js` file. These tests help you validate that the `getTimeAgoDescription()` function correctly returns the relative time descriptions for dates in the past. The tests generate specific sample input dates and confirm that the output matches the expected result.

## 2. Suggest bug fixes and changes to code snippets

Cody is very efficient at catching bugs and suggesting improvements to code snippets. To try this out, let's run our previously generated unit test and see if Cody notices any issues. Inside your VS Code terminal, run the following command to try the unit test:

```bash
node tests/test.js
```

This results in an error, as our function does not identify the `describe` statement.

![Example of running failed unit test ](https://storage.googleapis.com/sourcegraph-assets/Docs/cody-quickstart/unit-test-fail.png)

Let's paste this error into the Cody chat and see what suggestions it provides:

![Example of error debugging with Cody ](https://storage.googleapis.com/sourcegraph-assets/Docs/cody-quickstart/debug-with-cody.png)

Leveraging the failed test output, Cody is able to identify the potential bug and suggest a fix. It recommends installing `mocha`, importing it at the top of the `test.js` file to identify `describe`, and finally running it with `mocha`.

![Example of successfully running unit test ](https://storage.googleapis.com/sourcegraph-assets/Docs/cody-quickstart/passed-tests.png)

## 3. Ask Cody to add code documentation

Cody can also create well-crafted code documentation and comments, significantly improving the readability experience.
You can add it by selecting a code snippet and running the **Document** command. Cody will add a comment to the top of the code snippet.

<video width="1920" height="1080" loop playsinline controls style="width: 100%; height: auto; max-width: 50rem">
  <source src="https://storage.googleapis.com/sourcegraph-assets/Docs/Media/code-comments-cody.mp4" type="video/mp4">
</video>

## Try other commands and Cody chat

That's it for this quickstart guide! Feel free to continue chatting with Cody to learn more about its [capabilities](capabilities.md). Cody can run some other useful commands including:

- Explain code snippets
- Refactor code and variable names
- Translate code to different languages
- Answer general questions about your code
- Write context-aware code with a general awareness of the broader codebase

[cody-with-sourcegraph]: overview//cody-with-sourcegraph.md
