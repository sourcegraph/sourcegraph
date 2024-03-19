# Generate Unit Tests

<p class="subtitle">In this guide, you'll learn how to use Cody to generate unit tests for your codebase.</p>

Writing unit tests for your code is important for a robust and reliable codebase. These unit tests will help you ensure that your code works as expected and doesn't break in the future. Writing these tests can appear cumbersome, but Cody can automatically generate unit tests for your codebase.

This guide will teach you to generate a unit test using Cody with real-time code examples.

## Prerequisites

Before you begin, make sure you have the following:

- Access to Sourcegraph and are logged into your account
- Cody VS Code extension installed and connected to your Sourcegraph instance

> NOTE: Cody also provides extensions for [JetBrains](./../overview/install-jetbrains.md) and [Neovim](./../overview/install-neovim.md), which are not covered in this guide.

## Generate unit tests with Cody

One of the prominent commands in Cody is **Generate unit tests** (`/test`). All you need to do is select a code block and ask Cody to write the tests for you.

For this guide, let's use an example that converts any `given date` into a human-readable description of the time elapsed between the `given date` and the `current date`.

<video width="1920" height="1080" loop playsinline controls style="width: 100%; height: auto; max-width: 50rem">
  <source src="https://storage.googleapis.com/sourcegraph-assets/Docs/Media/unit-test-eg.mp4" type="video/mp4">
</video>

## Using the `/test` command

Cody offers a default `/test` command that you can use to generate unit tests for your codebase. Inside the Cody chat window, type `/test` and select the code block for which you want to generate a unit test. Hit **Enter**, and Cody will start writing a unit test for you.

> NOTE: The unit test command is only available in VS Code and JetBrains editor Cody extensions.

Let's create a unit test for the `getHumanReadableTime()` function in the `date.js` file.

```js
/**
 * Returns a human-readable string describing the time difference between the given date string and now.
 *
 * Calculates the difference in years, months, and days between the given date string and the current date.
 * Formats the time difference into a descriptive string like "2 years 3 months ago".
 * Handles singular/plural years/months/days.
 * If the given date is in the future, returns the string "today".
 */
export function getHumanReadableTime(dateString) {
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

For such an example, a unit test that checks the basic validation of expected output for recent dates, older dates, current date, and invalid inputs is required.

<video width="1920" height="1080" loop playsinline controls style="width: 100%; height: auto; max-width: 50rem">
  <source src="https://storage.googleapis.com/sourcegraph-assets/Docs/Media/write-test.mp4" type="video/mp4">
</video>

As shown in the demo above, Cody intelligently generated a unit test for you with just a single command. Also, note that with the available context, Cody analyzed the codebase and automatically learned that you are using Vite.

So, it tailored the test to be compatible with Vite's testing framework. This shows how Cody leverages context to provide relevant, framework-specific unit tests.

If the generated test does not meet your requirements, you can continue the chat thread with follow-up questions, and Cody will help you create a unit test of your choice.

## Trying out the generated test

Let's run the generated test to see whether it works or not. Inside your terminal, run the following command:

```bash
npm run test
```

All the tests should pass, and you should see the following output:

<video width="1920" height="1080" loop playsinline controls style="width: 100%; height: auto; max-width: 50rem">
  <source src="https://storage.googleapis.com/sourcegraph-assets/Docs/Media/run-test.mp4" type="video/mp4">
</video>

That's it! You've successfully generated a unit test for your codebase using Cody.
