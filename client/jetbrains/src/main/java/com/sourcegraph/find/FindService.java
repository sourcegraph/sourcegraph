package com.sourcegraph.find;

import com.intellij.ide.DataManager;
import com.intellij.openapi.Disposable;
import com.intellij.openapi.actionSystem.*;
import com.intellij.openapi.application.ApplicationManager;
import com.intellij.openapi.diagnostic.Logger;
import com.intellij.openapi.project.Project;
import com.intellij.openapi.wm.ex.WindowManagerEx;
import com.intellij.util.ui.UIUtil;
import org.cef.browser.CefBrowser;
import org.cef.handler.CefKeyboardHandler;
import org.cef.misc.BoolRef;
import org.jetbrains.annotations.NotNull;

import java.awt.*;
import java.awt.event.KeyEvent;
import java.awt.event.WindowEvent;

import static java.awt.event.InputEvent.ALT_DOWN_MASK;
import static java.awt.event.WindowEvent.WINDOW_GAINED_FOCUS;

public class FindService implements Disposable {
    private final Project project;
    private final FindPopupPanel mainPanel;
    private FindPopupDialog popup;
    private static final Logger logger = Logger.getInstance(FindService.class);

    public FindService(@NotNull Project project) {
        this.project = project;

        // Create main panel
        mainPanel = new FindPopupPanel(project);
    }

    synchronized public void showPopup() {
        if (popup != null && popup.isDisposed()) {
            System.out.println("DEADDEADDEADDEADDEADDEADDEADDEAD");
        }

        createOrShowPopup();

        // If the popup is already shown, hitting alt + a gain should behave the same as the native find in files
        // feature and focus the search field.
        if (mainPanel.getBrowser() != null) {
            mainPanel.getBrowser().focus();
        }
    }

    public void hidePopup() {
        popup.hide();
        hideMaterialUiOverlay();
    }

    @NotNull
    private void createOrShowPopup() {
        if (popup != null && !popup.isDisposed()) {
            popup.show();
        }
        if (popup == null || popup.isDisposed()) {
            if (popup != null && popup.isDisposed()) {
                System.out.println("???????");
            }
            popup = new FindPopupDialog(project, mainPanel);

            // For some reason, adding a cancelCallback will prevent the cancel event to fire when using the escape key. To
            // work around this, we add a manual listener to both the global key handler (since the editor component seems
            // to work around the default swing event hands long) and the browser panel which seems to handle events in a
            // separate queue.
            registerGlobalKeyListeners();
            registerJBCefClientKeyListeners();
            registerOutsideClickListener();
        }
    }

    private void registerGlobalKeyListeners() {
        KeyboardFocusManager.getCurrentKeyboardFocusManager()
            .addKeyEventDispatcher(e -> {
                if (e.getID() != KeyEvent.KEY_PRESSED || popup != null && (popup.isDisposed() || !popup.isVisible())) {
                    return false;
                }

                return handleKeyPress(false, e.getKeyCode(), e.getModifiersEx());
            });
    }

    private void registerJBCefClientKeyListeners() {
        if (mainPanel.getBrowser() == null) {
            logger.error("Browser panel is null");
            return;
        }

        mainPanel.getBrowser().getJBCefClient().addKeyboardHandler(new CefKeyboardHandler() {
            @Override
            public boolean onPreKeyEvent(CefBrowser browser, CefKeyEvent event, BoolRef is_keyboard_shortcut) {
                return false;
            }

            @Override
            public boolean onKeyEvent(CefBrowser browser, CefKeyEvent event) {
                return handleKeyPress(true, event.windows_key_code, event.modifiers);
            }
        }, mainPanel.getBrowser().getCefBrowser());
    }

    private boolean handleKeyPress(boolean isWebView, int keyCode, int modifiers) {
        if (keyCode == KeyEvent.VK_ESCAPE && modifiers == 0) {
            ApplicationManager.getApplication().invokeLater(this::hidePopup);
            return true;
        }


        if (!isWebView && keyCode == KeyEvent.VK_ENTER && (modifiers & ALT_DOWN_MASK) == ALT_DOWN_MASK) {
            if (mainPanel.getPreviewPanel() != null && mainPanel.getPreviewPanel().getPreviewContent() != null) {
                ApplicationManager.getApplication().invokeLater(() -> {
                    try {
                        mainPanel.getPreviewPanel().getPreviewContent().openInEditorOrBrowser();
                    } catch (Exception e) {
                        logger.error("Error opening file in editor", e);
                    }
                });
                return true;
            }
        }

        return false;
    }

    private void registerOutsideClickListener() {
        Window projectParentWindow = getParentWindow(null);

        Toolkit.getDefaultToolkit().addAWTEventListener(event -> {
            System.out.println(event);
            if (event instanceof WindowEvent) {
                WindowEvent windowEvent = (WindowEvent) event;

                // We only care for focus events
                if (windowEvent.getID() != WINDOW_GAINED_FOCUS) {
                    return;
                }

                // We only care for these events when the popup is shown
//                if (!this.popup.isVisible()) {
//                    return;
//                }

                System.out.println("-----------------------------------------------");
                System.out.println("getComponent(): " + windowEvent.getComponent().toString());
//                System.out.println(windowEvent.getComponent());
                System.out.println("projectParentWindow: " + projectParentWindow.toString());
//                System.out.println(projectParentWindow);
                System.out.println("getWindow(): " + this.popup.getWindow().toString());
//                System.out.println(this.popup.getWindow());

                // Detect if we're focusing the Sourcegraph popup
                if (windowEvent.getComponent().equals(this.popup.getWindow())) {
                    System.out.println("windowEvent.getComponent().equals(this.popup.getWindow())");
                    return;
                }
//                if (popup instanceof AbstractPopup) {
//                    Window sourcegraphPopupWindow = ((AbstractPopup) popup).getPopupWindow();
//
//                    if (windowEvent.getWindow().equals(sourcegraphPopupWindow)) {
//                        return;
//                    }
//                }

                // Detect if the newly focused window is a parent of the project root window
                Window currentProjectParentWindow = getParentWindow(windowEvent.getComponent());
                System.out.println("currentProjectParentWindow(): " + currentProjectParentWindow.toString());
//                System.out.println(currentProjectParentWindow);
                if (currentProjectParentWindow.equals(projectParentWindow)) {
                    System.out.println("currentProjectParentWindow.equals(projectParentWindow)");
                    hidePopup();
                }
            }
        }, AWTEvent.WINDOW_EVENT_MASK);
    }

    // https://sourcegraph.com/github.com/JetBrains/intellij-community@27fee7320a01c58309a742341dd61deae57c9005/-/blob/platform/platform-impl/src/com/intellij/ui/popup/AbstractPopup.java?L475-493
    private Window getParentWindow(Component component) {
        Window window = null;
        Component parent = UIUtil.findUltimateParent(component == null ? WindowManagerEx.getInstanceEx().getFocusedComponent(project) : component);
        if (parent instanceof Window) {
            window = (Window) parent;
        }
        if (window == null) {
            window = KeyboardFocusManager.getCurrentKeyboardFocusManager().getFocusedWindow();
        }
        return window;
    }

    @Override
    public void dispose() {
        if (popup != null) {
            popup.getWindow().dispose();
        }

        mainPanel.dispose();
    }


    // We manually emit an action defined by the material UI theme to hide the overlay it opens whenever a popover is
    // created. This third-party plugin does not work with our approach of keeping the popover alive and thus, when the
    // Sourcegraph popover is closed, their custom overlay stays active.
    //
    //   - https://github.com/sourcegraph/sourcegraph/issues/36479
    //   - https://github.com/mallowigi/material-theme-issues/issues/179
    private void hideMaterialUiOverlay() {
        AnAction materialAction = ActionManager.getInstance().getAction("MTToggleOverlaysAction");
        if (materialAction != null) {
            try {
                DataContext dataContext = DataManager.getInstance().getDataContextFromFocusAsync().blockingGet(10);
                if (dataContext != null) {
                    materialAction.actionPerformed(
                        new AnActionEvent(
                            null,
                            dataContext,
                            ActionPlaces.UNKNOWN,
                            new Presentation(),
                            ActionManager.getInstance(),
                            0)
                    );
                }
            } catch (Exception ignored) {
            }
        }
    }
}
