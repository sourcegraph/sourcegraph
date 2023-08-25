package com.sourcegraph.cody.config

import com.intellij.openapi.actionSystem.AnActionEvent
import com.intellij.openapi.actionSystem.PlatformCoreDataKeys
import com.intellij.openapi.project.DumbAwareAction
import com.intellij.openapi.project.Project
import com.intellij.openapi.util.NlsContexts
import java.awt.Component
import javax.swing.JComponent

class AddSourcegraphAccountWithTokenAction : BaseAddAccountWithTokenAction() {
    override val defaultServer: String
        get() = SourcegraphServerPath.DEFAULT_HOST
}

class AddSourcegraphEnterpriseAccountAction : BaseAddAccountWithTokenAction() {
    override val defaultServer: String
        get() = ""
}

abstract class BaseAddAccountWithTokenAction : DumbAwareAction() {
    abstract val defaultServer: String

    override fun update(e: AnActionEvent) {
        e.presentation.isEnabledAndVisible = e.getData(SourcegraphAccountsHost.KEY) != null
    }

    override fun actionPerformed(e: AnActionEvent) {
        val accountsHost = e.getData(SourcegraphAccountsHost.KEY)!!
        val dialog =
            newAddAccountDialog(
                e.project,
                e.getData(PlatformCoreDataKeys.CONTEXT_COMPONENT),
                accountsHost::isAccountUnique)

        dialog.setServer(defaultServer, defaultServer != SourcegraphServerPath.DEFAULT_HOST)
        if (dialog.showAndGet()) {
            accountsHost.addAccount(dialog.server, dialog.login, dialog.token)
        }
    }
}

private fun newAddAccountDialog(
    project: Project?,
    parent: Component?,
    isAccountUnique: UniqueLoginPredicate
): BaseLoginDialog =
    SourcegraphTokenLoginDialog(project, parent, isAccountUnique).apply {
        title = "Add Sourcegraph Account"
        setLoginButtonText("Add Account")
    }

internal class SourcegraphTokenLoginDialog(
    project: Project?,
    parent: Component?,
    isAccountUnique: UniqueLoginPredicate
) :
    BaseLoginDialog(
        project, parent, SourcegraphApiRequestExecutor.Factory.getInstance(), isAccountUnique) {

    init {
        title = "Login to Sourcegraph"
        setLoginButtonText("Login")
        loginPanel.setTokenUi()

        init()
    }

    internal fun setLoginButtonText(@NlsContexts.Button text: String) = setOKButtonText(text)

    override fun createCenterPanel(): JComponent = loginPanel.setPaddingCompensated()
}

