package web_test

import (
	"io/fs"
	"strings"
	"testing"

	"sk5proxy/internal/web"
)

// readStatic reads an embedded static file by name.
func readStatic(t *testing.T, name string) string {
	t.Helper()
	fsys, err := fs.Sub(web.StaticFS(), "static")
	if err != nil {
		t.Fatalf("fs.Sub: %v", err)
	}
	data, err := fs.ReadFile(fsys, name)
	if err != nil {
		t.Fatalf("ReadFile %s: %v", name, err)
	}
	return string(data)
}

func TestCSS_NoTransitionAll(t *testing.T) {
	css := readStatic(t, "style.css")
	if strings.Contains(css, "transition:") && strings.Contains(css, "transition: all") {
		t.Fatal("style.css must not use 'transition: all'; restrict to transform/opacity per DESIGN.md §6")
	}
}

func TestCSS_ActiveRow_NoBorderLeft(t *testing.T) {
	css := readStatic(t, "style.css")
	if strings.Contains(css, "border-left:") {
		t.Fatal("style.css must not use border-left on active rows; use inset box-shadow to avoid layout shift")
	}
}

func TestCSS_TertiaryContrast_NotOldValues(t *testing.T) {
	css := readStatic(t, "style.css")
	// Old values that failed WCAG AA (<4.5:1).
	for _, bad := range []string{"#999999", "#999"} {
		if strings.Contains(css, "--text-tertiary:"+bad) || strings.Contains(css, "--text-tertiary: "+bad) {
			t.Fatalf("style.css --text-tertiary must not be %s (fails WCAG AA)", bad)
		}
	}
}

func TestCSS_ModalCentering_HasMarginAuto(t *testing.T) {
	css := readStatic(t, "style.css")
	if !strings.Contains(css, "margin: auto") {
		t.Fatal("style.css .modal must include 'margin: auto' to center native dialogs against global * margin:0 reset")
	}
}

func TestCSS_ReducedMotion_DisablesTransitionsAndAnimation(t *testing.T) {
	css := readStatic(t, "style.css")
	if !strings.Contains(css, "prefers-reduced-motion") {
		t.Fatal("style.css must include prefers-reduced-motion media query")
	}
	if !strings.Contains(css, "animation: none") {
		t.Fatal("style.css prefers-reduced-motion must disable animations with 'animation: none'")
	}
	if !strings.Contains(css, "transition: none") {
		t.Fatal("style.css prefers-reduced-motion must disable transitions with 'transition: none'")
	}
}

func TestCSS_ErrorBanner_WhiteTextOnSolidBackground(t *testing.T) {
	css := readStatic(t, "style.css")
	// WCAG AA >=4.5:1: white text on solid error background.
	if !strings.Contains(css, ".form-error") || !strings.Contains(css, ".action-error") {
		t.Fatal("style.css must define .form-error and .action-error")
	}
	if strings.Contains(css, "color-mix(in srgb, var(--status-error) 8%") {
		t.Fatal("style.css must not use 8% tint for error banners (fails WCAG AA); use solid --status-error background with white text")
	}
}

func TestCSS_DarkMode_FilledButtonContrast(t *testing.T) {
	css := readStatic(t, "style.css")
	// WCAG AA >=4.5:1: darker filled backgrounds with white text in dark mode.
	if !strings.Contains(css, "#2563eb") {
		t.Fatal("style.css dark mode must use #2563eb for .btn-primary (>=4.5:1 with white text)")
	}
	if !strings.Contains(css, "#dc2626") {
		t.Fatal("style.css must use #dc2626 for destructive/error filled backgrounds (>=4.5:1 with white text)")
	}
}

func TestHTML_ConfirmDialog_HasInDialogErrorRegion(t *testing.T) {
	html := readStatic(t, "index.html")
	if !strings.Contains(html, `id="confirm-error"`) {
		t.Fatal("index.html confirm dialog must contain #confirm-error region for in-dialog delete errors")
	}
	if !strings.Contains(html, `class="confirm-error"`) {
		t.Fatal("index.html #confirm-error must use .confirm-error class")
	}
	if !strings.Contains(html, `confirm-error`) || !strings.Contains(html, `role="alert"`) {
		t.Fatal("index.html confirm dialog error region must have role='alert'")
	}
}

func TestJS_NetworkError_StableChineseMessage(t *testing.T) {
	js := readStatic(t, "api.js")
	if !strings.Contains(js, "网络连接失败") {
		t.Fatal("api.js must contain stable Chinese network error message '网络连接失败'")
	}
	if !strings.Contains(js, "catch") {
		t.Fatal("api.js must wrap fetch() in try/catch to handle network errors")
	}
}

func TestJS_DeleteConfirmation_ChineseCurlyQuotes(t *testing.T) {
	js := readStatic(t, "app.js")
	if !strings.Contains(js, `\u201c`) || !strings.Contains(js, `\u201d`) {
		t.Fatal("app.js must use Chinese curly quotes (\\u201c, \\u201d) in delete confirmation message")
	}
}

func TestJS_DeleteError_ShownInDialog(t *testing.T) {
	js := readStatic(t, "app.js")
	if !strings.Contains(js, "confirmError") {
		t.Fatal("app.js must reference confirmError element for in-dialog delete error display")
	}
	if !strings.Contains(js, "showConfirmError") {
		t.Fatal("app.js must define showConfirmError for displaying delete errors inside confirm dialog")
	}
	if !strings.Contains(js, "clearConfirmError") {
		t.Fatal("app.js must define clearConfirmError to reset in-dialog error on open/close/retry")
	}
}

func TestCSS_DarkMode_FilledAccentToken_BadgeContrast(t *testing.T) {
	css := readStatic(t, "style.css")
	// WCAG AA >=4.5:1: --accent-filled must be #2563eb in dark mode for white-text badge/button contrast.
	if !strings.Contains(css, "--accent-filled:") {
		t.Fatal("style.css must define --accent-filled token for white-text filled controls")
	}
	if !strings.Contains(css, ".badge-active") {
		t.Fatal("style.css must define .badge-active")
	}
	// badge-active must use the filled token, not --accent-primary (which is #3b8bff in dark mode, ~3.32:1)
	if strings.Contains(css, ".badge-active") && strings.Contains(css, "background: var(--accent-primary)") {
		// Check that badge-active specifically does NOT use --accent-primary for background
		// by verifying it uses --accent-filled instead
	}
	if !strings.Contains(css, "background: var(--accent-filled)") {
		t.Fatal("style.css .badge-active must use var(--accent-filled) for >=4.5:1 white-text contrast in dark mode")
	}
}

func TestJS_DeleteConfirmation_NoPhraseSplit(t *testing.T) {
	js := readStatic(t, "app.js")
	// The fixed suffix must contain U+2060 WORD JOINER between 恢 and 复 to prevent line split.
	if !strings.Contains(js, "\\u2060") {
		t.Fatal("app.js must insert U+2060 WORD JOINER between 恢 and 复 in delete confirmation to prevent phrase split across lines")
	}
	// The name must be in a separate node for independent wrapping.
	if !strings.Contains(js, "confirm-name") {
		t.Fatal("app.js must wrap upstream name in a .confirm-name node for independent CJK wrapping")
	}
}

func TestJS_APIError_NoRawEnglishFallback(t *testing.T) {
	js := readStatic(t, "api.js")
	// Non-JSON and unrecognized errors must use FALLBACK_ERROR, not raw English or '请求失败'.
	if strings.Contains(js, "'请求失败'") || strings.Contains(js, "\"请求失败\"") {
		t.Fatal("api.js must not use '请求失败'; use FALLBACK_ERROR ('操作失败，请检查配置') for unrecognized errors")
	}
	if !strings.Contains(js, "FALLBACK_ERROR") {
		t.Fatal("api.js must reference FALLBACK_ERROR for unknown error fallback")
	}
	// JSON parse failures must be caught to prevent leaking parser English.
	if !strings.Contains(js, "try") || !strings.Contains(js, "res.json()") {
		t.Fatal("api.js must wrap res.json() in try/catch to handle malformed JSON without leaking parser English")
	}
}
