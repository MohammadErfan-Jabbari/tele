// Package sanitize strips terminal control and invisible prompt-injection
// characters from untrusted Telegram strings while preserving Persian ZWNJ and
// emoji ZWJ sequences, and redacts Telegram login codes and gift links.
package sanitize

import (
	"regexp"
	"strings"
	"unicode"
)

const (
	telegramServiceID    int64 = 777000
	telegramServiceAltID int64 = 489000
	redactedLoginCode          = "[REDACTED_LOGIN_CODE]"
	redactedGiftLink           = "[REDACTED_GIFT_LINK]"
	zwnj                 rune  = '\u200c'
	zwj                  rune  = '\u200d'
)

var (
	ansiPattern      = regexp.MustCompile(`\x1b\][^\x07\x1b]*(?:\x07|\x1b\\)|\x1b\[[0-?]*[ -/]*[@-~]|\x1b[@-Z\\-_]`)
	giftLinkPattern  = regexp.MustCompile(`(?i)(?:https?://)?(?:t\.me|telegram\.me|telegram\.dog)/giftcode/[A-Za-z0-9_-]+|tg://giftcode\?(?:[^&\s]*&)*slug=[A-Za-z0-9_-]+`)
	loginCodePattern = regexp.MustCompile(`\b\d{5,6}\b`)
)

// Text strips ANSI escape sequences and Unicode Cc/Cf control characters while
// keeping newlines, tabs, Persian/Indic ZWNJ, and emoji ZWJ intact.
func Text(s string) string {
	if s == "" {
		return ""
	}
	cleaned := ansiPattern.ReplaceAllString(s, "")
	var b strings.Builder
	b.Grow(len(cleaned))
	for _, r := range cleaned {
		switch {
		case r == '\n' || r == '\t' || r == zwnj || r == zwj:
			b.WriteRune(r)
		case unicode.Is(unicode.Cc, r) || unicode.Is(unicode.Cf, r):
			continue
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

// MessageText sanitizes untrusted message text, redacts gift links for all
// senders, and redacts 5-to-6-digit login codes from Telegram service accounts.
func MessageText(senderID int64, s string) string {
	out := Text(s)
	out = giftLinkPattern.ReplaceAllString(out, redactedGiftLink)
	if senderID == telegramServiceID || senderID == telegramServiceAltID {
		out = loginCodePattern.ReplaceAllString(out, redactedLoginCode)
	}
	return out
}

// Name sanitizes single-line display labels such as chat titles, topic titles,
// sender names, and file names.
func Name(s string) string {
	out := Text(s)
	out = strings.ReplaceAll(out, "\n", " ")
	out = strings.ReplaceAll(out, "\t", " ")
	return strings.Join(strings.Fields(out), " ")
}
