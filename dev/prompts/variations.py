#!/usr/bin/env python3

# Basic prompt variation testing tool. Given a prompt generation function,
# input examples, and evaluation criteria, generates output from alternative
# prompts and asks the LLM to compare them, pairwise. Reports the (purported)
# best and worst prompt settings and example outputs.
#
# Inputs are configured in code--see comments below.

import asyncio
import collections
import math
import os
import requests
import sys

from concurrent.futures import ThreadPoolExecutor

# The Cody chat preamble. Include this in prompts for chat recipes.
# TODO: This should be kept in sync with the recipe preamble in client/cody-shared.
stock_cody_preamble = '''Human: You are Cody, an AI-powered coding assistant created by Sourcegraph. You work inside a text editor. You have access to my currently open files. You perform the following actions:
- Answer general programming questions.
- Answer questions about the code that I have provided to you.
- Generate code that matches a written description.
- Explain what a section of code does.`

In your responses, obey the following rules:
- Be as brief and concise as possible without losing clarity.
- All code snippets have to be markdown-formatted, and placed in-between triple backticks like this \`\`\`.
- Answer questions only if you know the answer or can make a well-informed guess. Otherwise, tell me you don't know and what context I need to provide you for you to answer the question.
- Only reference file names or URLs if you are sure they exist.

Assistant: Understood. I am Cody, an AI assistant made by Sourcegraph to help with programming tasks.
I work inside a text editor. I have access to your currently open files in the editor.
I will answer questions, explain code, and generate code as concisely and clearly as possible.
My responses will be formatted using Markdown syntax for code blocks.
I will acknowledge when I don't know an answer or need more context.

Human: '''

# Instructions for the LLM to evaluate the quality of a response.
# Edit this explaining what makes a good response.
evaluation_instructions = '''
The output must NOT contain <cody-help>.
The output MUST contain <cody-replace>.
The unchanged context MUST be repeated within the <cody-replace> tags.
The reply must NOT elide chunks of code using ...
'''

# Corpus of input examples. "input" are the dynamic parts which are
# spliced into your prompt templates. Replace all of these with
# examples specific to your use case.
#
# Each example must have:
# - 'id': a unique ID.
# - 'eval': the goal and evaluation criteria for the LLM to evaluate which response is better.
# - Any other input data consumed by your prompt generator (if any.) See generate_prompt below.
input_examples = [
    {
        'id': 'simple-comment',
        'eval': 'Document the "main" function.' + evaluation_instructions,
        'input': {
            'instruction': 'document what this program does',
            'filename': 'hello.rs',
            'prefix': '',
            'selected': '''
fn main() {
    println("Hello, world! From Cody demo land.")
}''',
            'suffix': ''
        },
    },
    {
        'id': 'complex-comment',
        'eval': 'Document the "digit" function.' + evaluation_instructions,
        'input': {
            'instruction': 'write a doc comment explaining what this function does',
            'filename': 'main.rs',
            'prefix': '''    if denominator != 0 && numerator % denominator == 0 {
        numerator / denominator
    } else {
        i32::MIN
    }
}

''',
            'selected': '''fn digit(n: u16) -> &'static str {
    match n {
        0b10 => ">1",
        0b100 => ">2",
        0b1000 => ">3",
        0b10000 => ">4",
        0b100000 => ">5",
        0b1000000 => ">6",
        0b10000000 => ">7",
        0b100000000 => ">8",
        0b1000000000 => ">9",
        _ if n.count_ones() == 0 => "!",
        _ if n.count_ones() == 1 => "?1",
        _ if n.count_ones() == 2 => "?2",
        _ if n.count_ones() == 3 => "?3",
        _ if n.count_ones() == 4 => "?4",
        _ if n.count_ones() == 5 => "?5",
        _ if n.count_ones() == 6 => "?6",
        _ if n.count_ones() == 7 => "?7",
        _ if n.count_ones() == 8 => "?8",
        _ if n.count_ones() == 9 => "?9",
        _ => panic!("unreachable"),
    }
}''',
            'suffix': '''
const DIM: usize = 9;
#[derive(Clone)]
struct Board {
    cells: [u16; DIM * DIM],
}

'''
        },
    },
    {
        'id': 'comment-fields',
        'eval': 'Added a comment describing "n" and a comment describing "i".' + evaluation_instructions,
        'input': {
            'instruction': 'add comments briefly explaining all the fields in this struct',
            'filename': 'main.rs',
            'prefix': '''    for receptor in &puzzle.receptors {
        for cell in &receptor.cells {
            let i = cell.v * DIM + cell.u;
            assert_eq!(false, used[i], "cell {},{} used twice", cell.u, cell.v);
            used[i] = true;
        }
    }
}

''',
            'selected': '',
            'suffix': '''// Iterates over the set bits in a board.
struct Bits {
    n: u16,
    i: usize,
}

// Yields the indices of the l.o. DIM bits in n which are set.
// We only need DIM because the puzzle is DIMxDIM
impl Bits {
    fn new(n: u16) -> Bits {
        Bits { n, i: 0 }
    }
}

impl Iterator for Bits {
    type Item = i32;
    fn next(&mut self) -> Option<Self::Item> {
        while self.i < 10 {
            let k = self.i;
            self.i += 1;
            if self.n & 1 << k != 0 {
                return Some(k as i32);
            }
        }
        None
    }
}
'''
        },
    },
    {
        'id': 'generate-react',
        'eval': 'Generated a React component with two color pickers for complementary colors.' + evaluation_instructions,
        'input': {
            'instruction': '''write a react component with a pair of color pickers
we want one of the colors to always be the complementary color of the other''',
            'filename': 'scheme.tsx',
            'prefix': '',
            'selected': '',
            'suffix': ''
        },
    },
    {
        'id': 'generate-method-body',
        'eval': 'Implement the complementaryColor function body.' + evaluation_instructions,
        'input': {
            'instruction': 'implement the function',
            'filename': 'scheme.tsx',
            'prefix': '''  const handleColor2Change = (e) => {
    setColor2(e.target.value);
    setColor1(complementaryColor(e.target.value));
  }

  return (
    <div>
      <ColorPicker color={color1} onChange={handleColor1Change} />
      <ColorPicker color={color2} onChange={handleColor2Change} />
    </div>
  )
}
''',
            'selected': '',
            'suffix': '''
function complementaryColor(color) {
  // Logic to calculate complementary color
}
'''
        },
    },
]

# Prompt generator. Consult the 'gen' oracle to pick the prompt to generate.
# Each invocation of gen.pick for a given choice must have the same set of
# alternatives. Alternatives can be strings, context generating functions, etc.
# Returns a function which, given an input example, returns a tuple of:
#
# - prompt text
# - context for the meta-evaluator
#
# TODO: For a given input example, the context should not depend on the prompt.
# If it does, update the candidate evaluation to handle varying contexts.
def generate_prompt(gen):
    prompt = []

    # Use gen.pick to construct variations of the prompt.
    #
    # Note, this program makes more than (|prompt space| * |num. examples|) ^ 2 LLM
    # calls, so don't test too many prompt variations in one go.
    prompt.append(gen.pick('FLATTER', ['', 'You are an expert software engineer who writes flawless code.', 'You have been known to make mistakes, but nobody is perfect.']))

    # When you find part of the prompt consistently helpful, hard-code that part.
    prompt.append('Fix up the <cody-help> tags.')
    #prompt.append(gen.pick('CODY-HELP', ['The area I need help with is highlighted with <cody-help> tags.', 'Fix up the <cody-help> tags.']))

    prompt.append('Strip the <cody-help> tags from your reply, just leave the improved code.')
    prompt.append('Put the replacement in <cody-replace> tags.')

    prompt.append(gen.pick('TAG-USE', ['You will repeat the code verbatim within the <cody-replace> tags.', 'Repeat all your unchanged code in the <cody-replace> tags.']))

    prompt.append('\n\nAssistant: OK, I understand. I will follow the prompts to improve the code, and only reply with code in <cody-replace> tags. The last thing I write will be the closing </cody-replace> tag. I will not write code outside <cody-replace> tags. I will not write the <cody-help> tags in my reply.\n\nHuman:')
    #prompt.append(gen.pick('ROBO-REPLY', ['', '\n\nAssistant: OK, I understand. I will follow the prompts to improve the code, and only reply with code in <cody-replace> tags. The last thing I write will be the closing </cody-replace> tag. I will not write code outside <cody-replace> tags. I will not write the <cody-help> tags in my reply.\n\nHuman:']))

    prompt_text = ' '.join(prompt)

    # Return a function which, given an input example, returns a tuple of:
    # - finished prompt
    # - context for the response evaluator (typically, just use your input context)
    def g(example):
        context = f"<cody-replace>{example['input']['prefix']}<cody-help prompt=\"{example['input']['instruction']}\">{example['input']['selected']}</cody-help>${example['input']['suffix']}</cody-replace>"
        return (stock_cody_preamble + prompt_text + f'''

This is part of the file you are writing, ${
            example['input']['filename']
        }:

{context}\n\nAssistant: ''', context)
    return g


# Set the ANTHROPIC_KEY environment variable for LLM access.
anthropic_key = os.getenv('ANTHROPIC_KEY')
if not anthropic_key:
    print('set ANTHROPIC_KEY and re-run')
    sys.exit(1)


# Configure the model, temperature parameters, etc. for your application.
# TODO: Separate the prompt submissions for generating prompt output,
# and meta-evaluation of the outputs.
def submit_prompt(prompt):
    """Submit a prompt to the Anthropic language model API."""
    url = "https://api.anthropic.com/v1/complete"
    headers = {
        "x-api-key": anthropic_key,
        "content-type": "application/json"
    }
    data = {
        "prompt": prompt,
        "model": "claude-v1.3",
        "max_tokens_to_sample": 1000,
        "temperature": 0.2,
        "stop_sequences": ["\n\nHuman: "]
    }
    response = requests.post(url, headers=headers, json=data)
    response.raise_for_status()
    return response.json()


# You should not need to modify below this point. (Improvements welcome! Human eval mode would be nice.)


class Schema(object):
    def __init__(self):
        self.dimensions = {}
        self.work = [Prompt(self, {})]
        self.prompts = []

    def discover_prompts(self, prompt_callback):
        while self.work:
            item = self.work.pop()
            callback = prompt_callback(item)
            item.id = len(self.prompts)
            item.generator = callback
            self.prompts.append(item)

    def advise(self, generator, label, n):
        if label in self.dimensions:
            assert self.dimensions[label] == n
        else:
            self.dimensions[label] = n
        if label not in generator.choices:
            for i in range(1, n):
                obj = generator.choices.copy()
                obj[label] = i
                self.work.append(Prompt(self, obj))
            generator.choices[label] = 0
        return generator.choices[label]


class Prompt(object):
    def __init__(self, schema, choices):
        self.schema = schema
        self.choices = choices or {}

    def pick(self, label, choices):
        return choices[self.schema.advise(self, label, len(choices))]

    def __repr__(self):
        return str(self.choices)


Sample = collections.namedtuple('Sample', ['prompt', 'input_example', 'context', 'output'])


def try_candidate(prompt, example):
    text, context = prompt.generator(example)
    result = submit_prompt(text)
    return Sample(prompt, example, context, result)


async def run_parallel_candidates(prompts):
    with ThreadPoolExecutor(max_workers=16) as executor:
        loop = asyncio.get_event_loop()
        return [loop.run_in_executor(executor, try_candidate, *prompt) for prompt in prompts]


# Evaluates whether A or B is better. Returns 1 if A is better, -1 if B is better.
def evaluate_candidates(goal, context, a, b):
    text = f'''I gave two people these high level instructions:

{goal}.

You are a researcher who will evaluate who performed the task better. I gave them this context:

> ```
+ '\n> '.join(context.splitlines())
```

Person A wrote:

{a}

Person B wrote:

{b}

The goal was: {goal}.

Who performed the task better? Let's think through this step by step.

Assistant: Person '''
    result = submit_prompt(text)
    completion: str = result['completion'].lstrip()
    person_a = completion.find('A ')
    person_b = completion.find('B ')
    if person_a == -1 and person_b == -1:
        score = 0
    elif person_a == -1:
        score = -1
    elif person_b == -1:
        score = 1
    else:
        score = (person_a < person_b and 1) or -1
    # To see the meta-evaluator output (it is cool!) uncomment:
    #print(completion)
    #print(score)
    #print('-' * 10)
    return score


class WinLossTable(object):
    def __init__(self, n):
        self.n = n
        self.scores = [0 for _ in range(n*n)]

    def add(self, a, b, value):
        self.scores[(a * self.n) + b] += value
        self.scores[(b * self.n) + a] -= value

    def show(self):
        try:
            n = 1 + math.ceil(math.log10(max(map(abs, self.scores))))
        except ValueError:
            n = 2
        print('   |', end='')
        for i in range(self.n):
            print(f' {i:-{n}d}', end='')
        print()
        print('-' * ((self.n + 1) * (n + 1) + 2))
        for i in range(self.n):
            print(f'{i:-3d}|', end='')
            for j in range(self.n):
                print(f' {self.scores[(i * self.n) + j]:-{n}d}', end='')
            print(f'| {self.marginal(i):-{n}d}')

    def marginal(self, i):
        return sum(self.scores[i*self.n:(i+1)*self.n])


def evaluate_and_score_candidates(table: WinLossTable, sample_a: Sample, sample_b: Sample):
    assert sample_a.input_example is sample_b.input_example
    assert sample_a.context == sample_b.context, 'could vary contexts but adjust evaluate_candidates to handle that fact'
    goal = sample_a.input_example['eval']
    score = evaluate_candidates(goal, sample_a.context, sample_a.output['completion'], sample_b.output['completion'])
    table.add(sample_a.prompt.id, sample_b.prompt.id, score)
    print(sample_a.prompt.id, sample_b.prompt.id, score)
    table.show()
    return score


async def run_parallel_evaluations(pairs):
    with ThreadPoolExecutor(max_workers=16) as executor:
        loop = asyncio.get_event_loop()
        return [loop.run_in_executor(executor, evaluate_and_score_candidates, *work) for work in pairs]


def main():
    print('Discovering prompts')
    schema = Schema()
    schema.discover_prompts(generate_prompt)
    for prompt in schema.prompts:
        print(f'{prompt.id}: {prompt.choices}')

    # If debugging, trim the prompts.
    #schema.prompts = schema.prompts[0:2]

    table = WinLossTable(len(schema.prompts))

    print('Generating candidate output')
    loop = asyncio.get_event_loop()
    corpus = list(map(lambda x: x.result(), loop.run_until_complete(run_parallel_candidates([(prompt, example) for prompt in schema.prompts for example in input_examples]))))

    # Collect samples by input.
    samples_by_input_example = collections.defaultdict(dict)
    for sample in corpus:
        samples_by_input_example[sample.input_example['id']][sample.prompt.id] = sample

    print('Comparing candidates')
    pairs = []
    for samples in samples_by_input_example.values():
        pairs.extend([(sA, sB) for sA in samples.values() for sB in samples.values() if sA is not sB])
    for result in loop.run_until_complete(run_parallel_evaluations([(table, *pair) for pair in pairs])):
        result.result()

    min_i = -1
    min_sum = 10**20
    max_i = -1
    max_sum = -10**20
    for i in range(table.n):
        i_sum = table.marginal(i)
        if i_sum > max_sum:
            max_sum = i_sum
            max_i = i
        if i_sum < min_sum:
            min_sum = i_sum
            min_i = i

    print(f'worst is {min_i} scoring {min_sum}')
    print('worst sample output')
    for sample in corpus:
        if sample.prompt.id == min_i:
            print(sample.output['completion'])
            print('-' * 20)

    print(f'best is {max_i} scoring {max_sum}')
    print('best sample output')
    for sample in corpus:
        if sample.prompt.id == max_i:
            print(sample.output['completion'])
            print('-' * 20)

    for prompt in schema.prompts:
        print(f'{prompt.id}: {prompt.choices}')
    table.show()


if __name__ == "__main__":
    main()
