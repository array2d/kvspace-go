package kvspace

import "strings"

// ── Mount ────────────────────────────────────────────────────────────────────

// NewMountValue 返回 mount XValue：kind="mount", raw=target 路径。
func NewMountValue(target string) XValue {
	return Raw(KindMount, []byte(target))
}

// DecodeMount 从 mount XValue 提取 target 路径。非 mount kind 返回空串。
func DecodeMount(v XValue) string {
	if v.Kind() != KindMount { return "" }
	return string(v.RawBytes())
}

// ── Overlay ──────────────────────────────────────────────────────────────────

// NewOverlayValue 返回 overlay XValue：kind="overlay", raw="wPath:rPath"。
func NewOverlayValue(wPath, rPath string) XValue {
	return Raw(KindOverlay, []byte(wPath+OverlaySep+rPath))
}

// DecodeOverlay 从 overlay XValue 提取 (wPath, rPath, ok)。
// 非 overlay kind 返回 ("", "", false)。
func DecodeOverlay(v XValue) (wPath, rPath string, ok bool) {
	if v.Kind() != KindOverlay { return "", "", false }
	w, r, found := strings.Cut(string(v.RawBytes()), OverlaySep)
	if !found { return "", "", false }
	return w, r, true
}
