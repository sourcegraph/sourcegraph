package tokenizer_test

import (
	"fmt"
	"testing"

	"github.com/hexops/autogold/v2"
	"github.com/stretchr/testify/require"

	"github.com/sourcegraph/sourcegraph/internal/completions/tokenizer"
)

var sampleTexts = []struct {
	Text       string
	WantTokens autogold.Value
}{
	{Text: `Many words map to one token, but some don't: indivisible.


Unicode characters like emojis may be split into many tokens containing the underlying bytes: 🤚🏾

Sequences of characters commonly found next to each other may be grouped together: 1234567890`,
		WantTokens: autogold.Expect(int(59)),
	},
	{Text: `Lorem Ipsum is simply dummy text of the printing and typesetting industry. Lorem Ipsum has been the industry's standard dummy text ever since the 1500s, when an unknown printer took a galley of type and scrambled it to make a type specimen book. It has survived not only five centuries, but also the leap into electronic typesetting, remaining essentially unchanged. It was popularised in the 1960s with the release of Letraset sheets containing Lorem Ipsum passages, and more recently with desktop publishing software like Aldus PageMaker including versions of Lorem Ipsum.`,
		WantTokens: autogold.Expect(int(118))},
	{Text: `

Human: Answer the following question only if you know the answer or can make a well-informed guess; otherwise tell me you don't know it.

What was the heaviest hippo ever recorded?

Assistant:`,
		WantTokens: autogold.Expect(int(48))},
	{Text: `

Human: I have two pet cats. One of them is missing a leg. The other one has a normal number of legs for a cat to have. In total, how many legs do my cats have?

Assistant: Can I think step-by-step?

Human: Yes, please do.

Assistant:`,
		WantTokens: autogold.Expect(int(70))},
	{Text: `There are many variations of passages of Lorem Ipsum available, but the majority have suffered alteration in some form, by injected humour, or randomised words which don't look even slightly believable. If you are going to use a passage of Lorem Ipsum, you need to be sure there isn't anything embarrassing hidden in the middle of text. All the Lorem Ipsum generators on the Internet tend to repeat predefined chunks as necessary, making this the first true generator on the Internet. It uses a dictionary of over 200 Latin words, combined with a handful of model sentence structures, to generate Lorem Ipsum which looks reasonable. The generated Lorem Ipsum is therefore always free from repetition, injected humour, or non-characteristic words etc.`,
		WantTokens: autogold.Expect(int(151))},
	{Text: `

Human: I want you to use a document and relevant quotes from the document to answer the question "{{QUESTION}}"

Here is the document, in <document></document> XML tags:
<document>
{{DOCUMENT}}
</document>

Here are direct quotes from the document that are most relevant to the question "{{QUESTION}}": {{QUOTES}}

Please use these to construct an answer to the question "{{QUESTION}}" as though you were answering the question directly. Ensure that your answer is accurate and doesn’t contain any information not directly supported by the document or the quotes.

Assistant:`,
		WantTokens: autogold.Expect(int(130))},
	{Text: `

Human: I am going to give you a sentence and you need to tell me how many times it contains the word “apple”. For example, if I say “I would like an apple” then the answer is “1” because the word “apple” is in the sentence once. You can reason through or explain anything you’d like before responding, but make sure at the very end, you end your answer with just the final answer in brackets, like this: [1].

Do you understand the instructions?

Assistant: Yes, I understand. For a given sentence, I should count how many times the word "apple" occurs in the sentence and provide the count as my response in brackets. For example, given the input "I would like an apple", my response should be "[1]".

Human: Correct. Here is the sentence: I need one apple to bake an apple pie, and another apple to keep for later.

Assistant:`,
		WantTokens: autogold.Expect(int(201))},
	{Text: `

Human: You are a customer service agent that is classifying emails by type.

Email:
<email>
Hi -- My Mixmaster4000 is producing a strange noise when I operate it. It also smells a bit smoky and plasticky, like burning electronics.  I need a replacement.
</email>

Categories are:
(A) Pre-sale question
(B) Broken or defective item
(C) Billing question
(D) Other (please explain)

Assistant: My answer is (`,
		WantTokens: autogold.Expect(int(114))},
}

func TestTokenize(t *testing.T) {
	tk, err := tokenizer.NewAnthropicClaudeTokenizer("testModel")
	require.NoError(t, err)

	for i, sample := range sampleTexts {
		sample := sample // copy
		t.Run(fmt.Sprintf("sample_%d", i), func(t *testing.T) {
			t.Parallel() // make sure tokenizer is concurrency-safe

			tokens, err := tk.Tokenize(sample.Text)
			require.NoError(t, err)
			sample.WantTokens.Equal(t, len(tokens))
		})
	}
}
