# Quickstart for Language Code Insights

Get started and create your first [language code insight](index.md) in 5 minutes or less.

## Introduction

> This quickstart guide assumes that **you have already completed [step 1, enabling the feature flag](quickstart.md#1-enable-the-experimental-feature-flag)**, on the main code insights quickstart guide. 

In this guide, you'll create a Sourcegraph language code insight that shows the percentage of lines of code in a repository by language. 

For more information about Code Insights see the [Code Insights](index.md) documentation. 

<img src="https://sourcegraphstatic.com/docs/images/code_insights/language_quickstart_insight_dark.png" class="screenshot">

### 1. Visit your Sourcegraph instance /Insights page and select "+ Create new insight" 

If you don't see the /Insights page you need to [enable code insights](quickstart.md#1-enable-the-experimental-feature-flag). 

### 2. On the insight type selection page, select "Create language usage insight"

This creates a code insight tracking how many lines of code you have in each language. 

If you are more interested tracking the historical or future result count of an arbitrary Sourcegraph search, [follow this tutorial](quickstart.md) instead. 

### 3. Once on the "Set up new language usage insight" form fields page, enter the repository you want to analyze. 

Enter repositories in the repository URL format, like `github.com/Sourcegraph/Sourcegraph`. 

The form field will validate that you've entered the repository correctly. 

In the current version, you can only set up a language insight for one repository at a time. (Want us to prioritize the ability to analyze multiple repositories in one pie chart? [Please do let us know](mailto:feedback@sourcegraph.com).)

### 4. Title your insight 

Title your language insight, generally something like the repository name or `Language usage in RepositoryName`. 

### 5. Set the 'Other' category threshold

This determines below what % the insight groups all other languages into an 'Other' languages category. 

Example: if the Threshold is set to 3%, and 2% of your code is in python and 2% of your code is in ruby, then both will be grouped into 'Other' and the value for 'Other' will be 4%. 

### 6. Click "create code insight" to view and save your insight

You'll be taken to the sourcegraph.example.com/insights page and can view your insight.
