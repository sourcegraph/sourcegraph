interface ChatProps extends ChatClassNames {
    messageInProgress: boolean;
}

export interface ExportedChatProps {
    messageInProgress: boolean;
}

/**
 * This is a docstring
 */
export const App: React.FunctionComponent<{ vscodeAPI: VSCodeWrapper }> = ({ vscodeAPI }) => {
    return (
        <div>Hello world</div>
    )
}

export const Chat: React.FunctionComponent<ChatProps> = ({
        messageInProgress,        messageBeingEdited,
}) => {
    return <div>Hello</div>
}

class Greeting extends Component {
    render() {
        return <h1>Hello, {this.props.name}!</h1>;
    }
}

export class ExportedGreeting extends React.Component {
    render() {
        return <h1>Hello, {this.props.name}!</h1>;
    }
}
