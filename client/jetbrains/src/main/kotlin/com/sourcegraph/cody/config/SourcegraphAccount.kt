package com.sourcegraph.cody.config

import com.intellij.collaboration.auth.ServerAccount
import com.intellij.openapi.util.NlsSafe
import com.intellij.util.xmlb.annotations.Attribute
import com.intellij.util.xmlb.annotations.Property
import com.intellij.util.xmlb.annotations.Tag
import com.intellij.util.xmlb.annotations.Transient
import com.sourcegraph.cody.localapp.LocalAppManager

@Tag("account")
class SourcegraphAccount(
    @set:Transient @NlsSafe @Attribute("name") override var name: String = "",
    @Property(style = Property.Style.ATTRIBUTE, surroundWithTag = false)
    override val server: SourcegraphServerPath =
        SourcegraphServerPath(LocalAppManager.DEFAULT_LOCAL_APP_URL),
    @Attribute("id") override val id: String = LocalAppManager.LOCAL_APP_ID
) : ServerAccount() {

  fun isCodyApp(): Boolean {
    return id == LocalAppManager.LOCAL_APP_ID
  }

  override fun toString(): String = "$server/$name"
}
