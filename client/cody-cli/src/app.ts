import { Command } from 'commander'
import figlet from 'figlet'

//add the following line
const program = new Command()

console.log(figlet.textSync('Cody CLI'))

program
    .version('0.0.1')
    .description('Cody CLI')
    .option('-p, --prompt <value>', 'Give Cody a prompt')
    .parse(process.argv)

const options = program.opts()
if (options.prompt !== '') {
    console.log('Prompt: ', options.prompt)
}
