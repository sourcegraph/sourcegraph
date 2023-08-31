package com.sourcegraph.cody.config

import com.intellij.collaboration.auth.ui.AccountsPanelFactory.accountsPanel
import com.intellij.collaboration.util.ProgressIndicatorsProvider
import com.intellij.ide.DataManager
import com.intellij.openapi.components.service
import com.intellij.openapi.options.BoundConfigurable
import com.intellij.openapi.project.Project
import com.intellij.openapi.ui.DialogPanel
import com.intellij.openapi.ui.ValidationInfo
import com.intellij.openapi.ui.setEmptyState
import com.intellij.openapi.util.Disposer
import com.intellij.ui.dsl.builder.MAX_LINE_LENGTH_NO_WRAP
import com.intellij.ui.dsl.builder.bindSelected
import com.intellij.ui.dsl.builder.bindText
import com.intellij.ui.dsl.builder.panel
import com.intellij.ui.dsl.gridLayout.HorizontalAlign
import com.intellij.ui.dsl.gridLayout.VerticalAlign
import com.sourcegraph.cody.Icons
import com.sourcegraph.config.ConfigUtil
import com.sourcegraph.config.PluginSettingChangeActionNotifier
import com.sourcegraph.config.PluginSettingChangeContext

class NewSettingsConfigurable(private val project: Project) :
    BoundConfigurable(ConfigUtil.SERVICE_DISPLAY_NAME) {
  private val codyProjectSettings = project.service<CodyProjectSettings>()
  private val codyApplicationSettings = service<CodyApplicationSettings>()
  private val settingsModel = SettingsModel()
  private val accountManager = service<SourcegraphAccountManager>()
  private val defaultAccountHolder = project.service<SourcegraphProjectDefaultAccountHolder>()

  override fun createPanel(): DialogPanel {
    val accountsModel = SourcegraphAccountListModel(project)
    val indicatorsProvider =
        ProgressIndicatorsProvider().also { Disposer.register(disposable!!, it) }
    val detailsProvider =
        SourcegraphAccounDetailsProvider(indicatorsProvider, accountManager, accountsModel)
    return panel {
      group("Authentication") {
        row {
          accountsPanel(
                  accountManager,
                  defaultAccountHolder,
                  accountsModel,
                  detailsProvider,
                  disposable!!,
                  true,
                  Icons.CodyLogo)
              .horizontalAlign(HorizontalAlign.FILL)
              .verticalAlign(VerticalAlign.FILL)
              .also {
                DataManager.registerDataProvider(it.component) { key ->
                  if (SourcegraphAccountsHost.KEY.`is`(key)) accountsModel else null
                }
              }
        }
        row {
          textField()
              .label("Custom request headers:")
              .comment(
                  """Any custom headers to send with every request to Sourcegraph.<br>
                  |Use any number of pairs: "header1, value1, header2, value2, ...".<br>
                  |Whitespace around commas doesn't matter.
              """
                      .trimMargin(),
                  MAX_LINE_LENGTH_NO_WRAP)
              .horizontalAlign(HorizontalAlign.FILL)
              .bindText(settingsModel::customRequestHeaders)
              .applyToComponent {
                this.setEmptyState("Client-ID, client-one, X-Extra, some metadata")
              }
              .validation {
                if (it.getText().isEmpty()) {
                  return@validation null
                }
                val pairs: Array<String> =
                    it.getText().split(",".toRegex()).dropLastWhile { it.isEmpty() }.toTypedArray()
                if (pairs.size % 2 != 0) {
                  return@validation ValidationInfo(
                      "Must be a comma-separated list of string pairs", it)
                }
                var i = 0
                while (i < pairs.size) {
                  val headerName = pairs[i].trim { it <= ' ' }
                  if (!headerName.matches("[\\w-]+".toRegex())) {
                    return@validation ValidationInfo("Invalid HTTP header name: $headerName", it)
                  }
                  i += 2
                }
                return@validation null
              }
        }
      }
      group("Cody AI") {
        row {
          checkBox("Enable Cody")
              .comment(
                  "Disable this to turn off all AI-based functionality of the plugin, including the Cody chat sidebar and autocomplete",
                  MAX_LINE_LENGTH_NO_WRAP)
              .bindSelected(settingsModel::isCodyEnabled)
        }
        row {
          checkBox("Enable Cody autocomplete")
              .bindSelected(settingsModel::isCodyAutocompleteEnabled)
        }
        row {
          checkBox("Enable debug")
              .comment("Enables debug output visible in the idea.log")
              .bindSelected(settingsModel::isCodyDebugEnabled)
        }
        row {
          checkBox("Verbose debug")
              .bindSelected(settingsModel::isCodyVerboseDebugEnabled)
        }
      }
      group("Code search") {
        row {
          textField()
              .label("Default branch name:")
              .comment("The branch to use if the current branch is not yet pushed")
              .horizontalAlign(HorizontalAlign.FILL)
              .bindText(settingsModel::defaultBranchName)
              .applyToComponent {
                this.setEmptyState("main")
                toolTipText = "Usually \"main\" or \"master\", but can be any name"
              }
        }
        row {
          textField()
              .label("Remote URL replacements:")
              .comment(
                  """You can replace specified strings in your repo's remote URL. <br>
                      |Use any number of pairs: "search1, replacement1, search2, replacement2, ...". <br>
                      |Pairs are replaced from left to right. Whitespace around commas doesn't matter.
                  """
                      .trimMargin(),
                  MAX_LINE_LENGTH_NO_WRAP)
              .horizontalAlign(HorizontalAlign.FILL)
              .bindText(settingsModel::remoteUrlReplacements)
              .applyToComponent {
                this.setEmptyState("search1, replacement1, search2, replacement2, ...")
              }
        }
        row {
          checkBox("Do not show the \"No Sourcegraph URL set\" notification for this project")
              .bindSelected(settingsModel::isUrlNotificationDismissed)
        }
      }
    }
  }

  override fun reset() {
    super.reset()
    settingsModel.isCodyEnabled = codyApplicationSettings.isCodyEnabled
    settingsModel.isCodyAutocompleteEnabled =
        codyApplicationSettings.isCodyAutocompleteEnabled
    settingsModel.isCodyDebugEnabled = codyApplicationSettings.isCodyDebugEnabled
    settingsModel.isCodyVerboseDebugEnabled =
        codyApplicationSettings.isCodyVerboseDebugEnabled
    settingsModel.isUrlNotificationDismissed =
        codyApplicationSettings.isUrlNotificationDismissed
    settingsModel.defaultBranchName = codyProjectSettings.defaultBranchName
    settingsModel.customRequestHeaders = codyProjectSettings.customRequestHeaders
    settingsModel.remoteUrlReplacements = codyProjectSettings.remoteUrlReplacements
  }

  override fun apply() {
    val bus = project.messageBus
    val publisher = bus.syncPublisher(PluginSettingChangeActionNotifier.TOPIC)

    val oldCodyEnabled = codyApplicationSettings.isCodyEnabled
    val oldCodyAutocompleteEnabled = codyApplicationSettings.isCodyAutocompleteEnabled
    val oldDefaultAccount = defaultAccountHolder.account
    val oldUrl = oldDefaultAccount?.server?.url ?: ""
    val oldAccessToken = oldDefaultAccount?.let { accountManager.findCredentials(it) }

    super.apply()

    val defaultAccount = defaultAccountHolder.account
    val accessToken = defaultAccount?.let { accountManager.findCredentials(it) }
    val newUrl = defaultAccount?.server?.url ?: ""
    val context =
        PluginSettingChangeContext(
            oldCodyEnabled,
            oldCodyAutocompleteEnabled,
            oldUrl,
            newUrl,
            oldUrl != newUrl || oldAccessToken != accessToken,
            settingsModel.isCodyEnabled,
            settingsModel.isCodyAutocompleteEnabled)

    codyProjectSettings.customRequestHeaders = settingsModel.customRequestHeaders
    codyProjectSettings.defaultBranchName = settingsModel.defaultBranchName
    codyProjectSettings.remoteUrlReplacements = settingsModel.remoteUrlReplacements
    codyApplicationSettings.isCodyEnabled = settingsModel.isCodyEnabled
    codyApplicationSettings.isCodyAutocompleteEnabled =
        settingsModel.isCodyAutocompleteEnabled
    codyApplicationSettings.isCodyDebugEnabled = settingsModel.isCodyDebugEnabled
    codyApplicationSettings.isCodyVerboseDebugEnabled =
        settingsModel.isCodyVerboseDebugEnabled
    codyApplicationSettings.isUrlNotificationDismissed =
        settingsModel.isUrlNotificationDismissed

    publisher.afterAction(context)
  }
}
