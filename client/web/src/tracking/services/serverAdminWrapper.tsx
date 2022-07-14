import { logEvent } from '../../user/settings/backend'

class ServerAdminWrapper {
    constructor() {
        // ServerAdminWrapper is never teared down
    }

    public trackPageView(eventAction: string, eventProperties?: any, publicArgument?: any): void {
        logEvent(eventAction, eventProperties, publicArgument)
    }

    public trackAction(eventAction: string, eventProperties?: any, publicArgument?: any): void {
        logEvent(eventAction, eventProperties, publicArgument)
    }
}

export const serverAdmin = new ServerAdminWrapper()
