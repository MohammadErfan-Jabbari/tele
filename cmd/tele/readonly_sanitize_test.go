package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/sorokin-vladimir/tele/internal/core"
	"github.com/sorokin-vladimir/tele/internal/core/project"
	"github.com/sorokin-vladimir/tele/internal/domain"
)

func TestCLI_TeleReadOnlyRefusesMutatingAndServiceCommandsBeforeDialing(t *testing.T) {
	t.Setenv("TELE_READ_ONLY", "1")

	for _, args := range [][]string{
		{"send", "me", "hello"},
		{"send", "me"},
		{"show", "me", "--mark-read"},
		{"daemon", "install"},
		{"daemon", "uninstall"},
	} {
		cmd := newRootCmd()
		cmd.SetArgs(args)
		err := cmd.Execute()
		var ec *exitCode
		if !errors.As(err, &ec) || ec.code != commandsOffCode {
			t.Fatalf("args %v: got err=%v, want exitCode %d", args, err, commandsOffCode)
		}
	}
}

func TestCLI_SanitizesControlCharsAndRedactsSecretsInOutput(t *testing.T) {
	var out bytes.Buffer
	msgs := []domain.Message{
		{
			ID:         1,
			Date:       at(25, 10, 0),
			SenderID:   777000,
			SenderName: "\x1b[31mTelegram\x1b[0m",
			Text:       "Login code: 54321 and \u202eمی\u200cخواهم https://t.me/giftcode/Secret123",
		},
	}
	if err := showJSON(&out, msgs, me); err != nil {
		t.Fatal(err)
	}
	var got messageJSON
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Sender != "Telegram" {
		t.Fatalf("sender=%q, want Telegram", got.Sender)
	}
	if bytes.Contains([]byte(got.Text), []byte("54321")) || bytes.Contains([]byte(got.Text), []byte("Secret123")) {
		t.Fatalf("leaked sensitive token in text: %q", got.Text)
	}
	if !bytes.Contains([]byte(got.Text), []byte("می\u200cخواهم")) {
		t.Fatalf("Persian ZWNJ lost in text: %q", got.Text)
	}

	var chatsOut bytes.Buffer
	if err := chatsJSON(&chatsOut, []project.ChatRow{{ID: 1, Title: "\x1b]0;pwn\x07Work\nChat", Kind: project.ChatKindUser}}); err != nil {
		t.Fatal(err)
	}
	var c chatJSON
	if err := json.Unmarshal(chatsOut.Bytes(), &c); err != nil {
		t.Fatal(err)
	}
	if c.Title != "Work Chat" {
		t.Fatalf("chat title=%q, want Work Chat", c.Title)
	}

	folders := foldersText([]domain.FolderFilter{{ID: 2, Title: "\x1b[31mNews\u202e\nFeed"}})
	if strings.Contains(folders, "\x1b") || strings.Contains(folders, "\u202e") || !strings.Contains(folders, "2 News Feed") {
		t.Fatalf("unsanitized folder title: %q", folders)
	}

	acc := newFakeAccount()
	acc.byName = map[core.ChatName]project.ChatRow{{ID: 42}: {ID: 42, Title: "\x1b[31mEvil\u202eChat", IsForum: false}}
	err := showMessages(shortCtx(t), held(acc), &bytes.Buffer{}, "42", showQuery{limit: 5, topic: 3})
	if err == nil || strings.Contains(err.Error(), "\x1b") || strings.Contains(err.Error(), "\u202e") || !strings.Contains(err.Error(), "EvilChat") {
		t.Fatalf("unsanitized error title: %v", err)
	}
}
