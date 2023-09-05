package com.sourcegraph.cody.config

import com.intellij.collaboration.auth.AccountsRepository
import com.intellij.openapi.components.PersistentStateComponent
import com.intellij.openapi.components.SettingsCategory
import com.intellij.openapi.components.State
import com.intellij.openapi.components.Storage
import com.sourcegraph.cody.localapp.LocalAppManager

@State(
    name = "CodyAccounts",
    storages =
        [
            Storage(value = "cody_accounts.xml"),
        ],
    reportStatistic = false,
    category = SettingsCategory.TOOLS)
class CodyPersisentAccounts :
    AccountsRepository<CodyAccount>, PersistentStateComponent<Array<CodyAccount>> {
  private var state = emptyArray<CodyAccount>()

  override var accounts: Set<CodyAccount>
    get() = state.toSet()
    set(value) {
      state = value.toTypedArray()
    }

  override fun getState(): Array<CodyAccount> = state

  override fun loadState(state: Array<CodyAccount>) {
    var finalState = state
    if (state.none { it.id == LocalAppManager.LOCAL_APP_ID }) {
      val localAppInstalled = LocalAppManager.isLocalAppInstalled()
      if (localAppInstalled) {
        finalState =
            state +
                CodyAccount.create(
                    LocalAppManager.LOCAL_APP_ID,
                    SourcegraphServerPath(LocalAppManager.getLocalAppUrl()),
                    LocalAppManager.LOCAL_APP_ID)
      }
    }
    this.state = finalState
  }
}
