const instruction_prompt = `Please follow these rules when answering my questions:
- Your answers and suggestions should based on the shared context only.
- Do not suggest anything that would break the working code.
- Do not make any assumptions and provide any misleading or hypothetical examples.
- All generated code should be full workable code.

<questions>{humanInput}</questions>
`
const prevent_hallucinations =
    "Answer the questions only if you know the answer or can make a well-informed guess, else tell me you don't know it."

export const answers = {
    terminal: 'Noted. I will answer your next question based on this terminal output with other code you shared.',
    selection: 'Noted. I will refer to this code you selected in the editor to answer your question.',
    file: 'Noted. I will refer to this codebase file you are looking at to answer you next question for the code in the <selected> tags.',
    fileList:
        'Noted. I will refer to this list of files from the {fileName} directory of your codebase to answer your next question.',
    packageJson: 'Noted. I will use the right libraries/framework already setup in your codebase for your questions.',
}

export const prompts = {
    instruction: instruction_prompt,
}

export const rules = {
    hallucination: prevent_hallucinations,
}

export const displayFileName = `\n
    File: `
