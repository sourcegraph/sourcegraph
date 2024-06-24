export enum PageRoutes {
    Index = '/',
    Search = '/search',
    SearchConsole = '/search/console',
    SignIn = '/sign-in',
    SignUp = '/sign-up',
    PostSignUp = '/post-sign-up',
    UnlockAccount = '/unlock-account/:token',
    Settings = '/settings',
    User = '/user/*',
    Organizations = '/organizations/*',
    SiteAdmin = '/site-admin/*',
    SiteAdminInit = '/site-admin/init',
    PasswordReset = '/password-reset',
    ApiConsole = '/api/console',
    UserArea = '/users/:username/*',
    Survey = '/survey/:score?',
    Extensions = '/extensions',
    Help = '/help/*',
    Debug = '/-/debug/*',
    RepoContainer = '/*',
    SetupWizard = '/setup',
    Teams = '/teams/*',
    RequestAccess = '/request-access/*',
    BatchChanges = '/batch-changes/*',
    CodeMonitoring = '/code-monitoring/*',
    Insights = '/insights/*',
    SearchJobs = '/search-jobs/*',
    Contexts = '/contexts',
    CreateContext = '/contexts/new',
    EditContext = '/contexts/:specOrOrg/:spec?/edit',
    Context = '/contexts/:specOrOrg/:spec?',
    NotebookCreate = '/notebooks/new',
    Notebook = '/notebooks/:id',
    Notebooks = '/notebooks',
    SearchNotebook = '/search/notebook',
    Cody = '/cody',
    CodyChat = '/cody/chat',
    CodySwitchAccount = '/cody/switch-account/:username',
    Own = '/own',
}

export enum CommunityPageRoutes {
    Kubernetes = '/kubernetes',
    Stackstorm = '/stackstorm',
    Temporal = '/temporal',
    O3de = '/o3de',
    ChakraUI = '/chakraui',
    Stanford = '/stanford',
    Cncf = '/cncf',
    Julia = '/julia',
    Backstage = '/backstage',
}
