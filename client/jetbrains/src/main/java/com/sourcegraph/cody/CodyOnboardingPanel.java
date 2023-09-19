package com.sourcegraph.cody;

import com.intellij.ide.ui.laf.darcula.ui.DarculaButtonUI;
import com.intellij.openapi.ui.VerticalFlowLayout;
import com.intellij.ui.ColorUtil;
import com.intellij.ui.SeparatorComponent;
import com.intellij.util.ui.JBUI;
import com.intellij.util.ui.UIUtil;
import com.sourcegraph.cody.chat.UIComponents;
import com.sourcegraph.cody.localapp.LocalAppManager;
import com.sourcegraph.cody.ui.HtmlViewer;
import java.awt.*;
import javax.swing.*;
import javax.swing.border.Border;

public class CodyOnboardingPanel extends JPanel {

  private static final int PADDING = 20;
  // 10 here is the default padding from the styles of the h2 and we want to make the whole padding
  // to be 20, that's why we need the difference between our PADDING and the default padding of the
  // h2
  private static final int ADDITIONAL_PADDING_FOR_HEADER = PADDING - 10;

  public CodyOnboardingPanel() {
    JEditorPane jEditorPane = HtmlViewer.createHtmlViewer(UIUtil.getPanelBackground());
    jEditorPane.setText(
        "<html><body><h2>Hi</h2>"
            + "<p>Let's start by getting you familiar with all the possibilities Cody provides:</p>"
            + "</body></html>");
    JButton runCodyAppButton = UIComponents.createMainButton("Open Cody App");
    runCodyAppButton.putClientProperty(DarculaButtonUI.DEFAULT_STYLE_KEY, Boolean.TRUE);
    runCodyAppButton.addActionListener(
        e -> {
          LocalAppManager.runLocalApp();
        });
    JPanel panelWithTheMessage = new JPanel();
    panelWithTheMessage.setLayout(new BoxLayout(panelWithTheMessage, BoxLayout.Y_AXIS));
    jEditorPane.setMargin(JBUI.emptyInsets());
    Border paddingInsideThePanel =
        JBUI.Borders.empty(ADDITIONAL_PADDING_FOR_HEADER, PADDING, 0, PADDING);
    JLabel hiImCodyLabel = new JLabel(Icons.HiImCody);
    JPanel hiImCodyPanel = new JPanel(new FlowLayout(FlowLayout.LEFT, 5, 0));
    hiImCodyPanel.add(hiImCodyLabel);
    panelWithTheMessage.add(hiImCodyPanel);
    panelWithTheMessage.add(jEditorPane);
    panelWithTheMessage.setBorder(paddingInsideThePanel);
    JPanel separatorPanel = new JPanel(new BorderLayout());
    separatorPanel.setBorder(JBUI.Borders.empty(PADDING, 0));
    SeparatorComponent separatorComponent =
        new SeparatorComponent(
            3, ColorUtil.brighter(UIUtil.getPanelBackground(), 3), UIUtil.getPanelBackground());
    separatorPanel.add(separatorComponent);
    panelWithTheMessage.add(separatorPanel);
    JPanel buttonPanel = new JPanel(new BorderLayout());
    buttonPanel.add(runCodyAppButton, BorderLayout.CENTER);
    buttonPanel.setOpaque(false);
    panelWithTheMessage.add(buttonPanel);
    this.setLayout(new VerticalFlowLayout(VerticalFlowLayout.TOP, 0, 0, true, false));
    this.setBorder(JBUI.Borders.empty(PADDING));
    this.add(panelWithTheMessage);
  }
}
