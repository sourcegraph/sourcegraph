package com.sourcegraph.cody.config

import com.intellij.openapi.progress.ProgressIndicator
import com.intellij.openapi.ui.ValidationInfo
import com.intellij.ui.DocumentAdapter
import com.intellij.ui.components.JBTextField
import com.intellij.ui.components.fields.ExtendableTextField
import com.intellij.ui.layout.ComponentPredicate
import com.intellij.ui.layout.LayoutBuilder
import com.sourcegraph.cody.config.DialogValidationUtils.notBlank
import java.net.UnknownHostException
import javax.swing.JComponent
import javax.swing.JTextField
import javax.swing.event.DocumentEvent

internal class SourcegraphTokenCredentialsUi(
    private val serverTextField: ExtendableTextField,
    val factory: SourcegraphApiRequestExecutor.Factory,
    val isAccountUnique: UniqueLoginPredicate
) : SourcegraphCredentialsUi() {

  private val tokenTextField = JBTextField()
  private var fixedLogin: String? = null

  fun setToken(token: String) {
    tokenTextField.text = token
  }

  override fun LayoutBuilder.centerPanel() {
    row("Server: ") { serverTextField(pushX, growX) }
    row("Token: ") { tokenTextField(constraints = arrayOf(pushX, growX)) }
  }

  override fun getPreferredFocusableComponent(): JComponent = tokenTextField

  override fun getValidator(): Validator = { notBlank(tokenTextField, "Token cannot be empty") }

  override fun createExecutor() = factory.create(tokenTextField.text)

  override fun acquireLoginAndToken(
      server: SourcegraphServerPath,
      executor: SourcegraphApiRequestExecutor,
      indicator: ProgressIndicator
  ): Pair<String, String> {
    val login = acquireLogin(server, executor, indicator, isAccountUnique, fixedLogin)
    return login to tokenTextField.text
  }

  override fun handleAcquireError(error: Throwable): ValidationInfo =
      when (error) {
        is SourcegraphParseException ->
            ValidationInfo(error.message ?: "Invalid server url", serverTextField)
        else -> handleError(error)
      }

  override fun setBusy(busy: Boolean) {
    tokenTextField.isEnabled = !busy
  }

  fun setFixedLogin(fixedLogin: String?) {
    this.fixedLogin = fixedLogin
  }

  companion object {

    fun acquireLogin(
        server: SourcegraphServerPath,
        executor: SourcegraphApiRequestExecutor,
        indicator: ProgressIndicator,
        isAccountUnique: UniqueLoginPredicate,
        fixedLogin: String?
    ): String {
      val accountDetails =
          SourcegraphSecurityUtil.loadCurrentUserDetails(executor, indicator, server)

      val login = accountDetails.username
      if (fixedLogin != null && fixedLogin != login)
          throw SourcegraphAuthenticationException("Token should match username \"$fixedLogin\"")
      if (!isAccountUnique(login, server)) throw LoginNotUniqueException(login)

      return login
    }

    fun handleError(error: Throwable): ValidationInfo =
        when (error) {
          is LoginNotUniqueException ->
              ValidationInfo("Account '${error.login}' already added").withOKEnabled()
          is UnknownHostException -> ValidationInfo("Server is unreachable").withOKEnabled()
          is SourcegraphAuthenticationException ->
              ValidationInfo("Incorrect credentials.\n" + error.message.orEmpty()).withOKEnabled()
          else ->
              ValidationInfo("Invalid authentication data.\n" + error.message.orEmpty())
                  .withOKEnabled()
        }
  }
}

private val JTextField.serverValid: ComponentPredicate
  get() =
      object : ComponentPredicate() {
        override fun invoke(): Boolean = tryParseServer() != null

        override fun addListener(listener: (Boolean) -> Unit) =
            document.addDocumentListener(
                object : DocumentAdapter() {
                  override fun textChanged(e: DocumentEvent) = listener(tryParseServer() != null)
                })
      }

private fun JTextField.tryParseServer(): SourcegraphServerPath? =
    try {
      SourcegraphServerPath.from(text.trim())
    } catch (e: SourcegraphParseException) {
      null
    }
