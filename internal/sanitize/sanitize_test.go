package sanitize

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestText_StripsControlAndBidiButPreservesPersianZWNJAndEmojiZWJ(t *testing.T) {
	// Persian "می‌خواهم" uses ZWNJ (\u200c); family emoji uses ZWJ (\u200d).
	persian := "می\u200cخواهم\nسلام\tدوست"
	assert.Equal(t, persian, Text(persian))

	emoji := "👨\u200d👩\u200d👧"
	assert.Equal(t, emoji, Text(emoji))

	// ANSI CSI, OSC, C0 control, bidi override (\u202e), zero-width space (\u200b) stripped.
	dirty := "\x1b[31mhi\x1b[0m\x1b]8;;https://evil.example\x07click\x1b]8;;\x07\u202eabc\u200b\x00"
	assert.Equal(t, "hiclickabc", Text(dirty))
}

func TestMessageText_RedactsTelegramLoginCodesAndGiftLinks(t *testing.T) {
	// Service account 777000 login code redacted.
	loginMsg := "Login code: 12345. Do not give this code to anyone."
	got := MessageText(777000, loginMsg)
	assert.NotContains(t, got, "12345")
	assert.Contains(t, got, "[REDACTED_LOGIN_CODE]")

	// Service account 489000 login code redacted.
	loginMsg2 := "Your code is 654321"
	got2 := MessageText(489000, loginMsg2)
	assert.NotContains(t, got2, "654321")
	assert.Contains(t, got2, "[REDACTED_LOGIN_CODE]")

	// Ordinary user 5-digit numbers (e.g. postal code / price) are NOT redacted.
	normal := MessageText(1234567, "Meet at 28001 Madrid")
	assert.Equal(t, "Meet at 28001 Madrid", normal)

	// Gift links redacted for every sender, across Telegram domains and query parameter order.
	gift := MessageText(1234567, "Claim https://t.me/giftcode/AbC123_xyz or https://telegram.me/giftcode/Me456 or telegram.dog/giftcode/Dog789 or tg://giftcode?foo=1&slug=SecretSlug99 now")
	assert.NotContains(t, gift, "AbC123_xyz")
	assert.NotContains(t, gift, "Me456")
	assert.NotContains(t, gift, "Dog789")
	assert.NotContains(t, gift, "SecretSlug99")
	assert.Contains(t, gift, "[REDACTED_GIFT_LINK]")
}

func TestName_CollapsesWhitespaceAndStripsControls(t *testing.T) {
	dirty := "\x1b[32mAlice\nBob\t\u202e\x07"
	assert.Equal(t, "Alice Bob", Name(dirty))
}
