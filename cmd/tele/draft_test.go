package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/sorokin-vladimir/tele/internal/core"
	"github.com/sorokin-vladimir/tele/internal/core/project"
	"github.com/sorokin-vladimir/tele/internal/domain"
)

type draftAsk struct {
	h    domain.HistoryKey
	text string
}

func (f *fakeAccount) SaveDraft(_ context.Context, h domain.HistoryKey, text string) error {
	f.savedDrafts = append(f.savedDrafts, draftAsk{h, text})
	return f.draftErr
}

func TestDraftSetAndClear(t *testing.T) {
	acc := newFakeAccount()
	acc.byName = map[core.ChatName]project.ChatRow{
		{Username: "bob"}:     {ID: 42, Title: "Bob"},
		{Username: "gophers"}: {ID: 300, Title: "Gophers", IsForum: true},
	}

	var out bytes.Buffer
	if err := saveDraft(shortCtx(t), acc, &out, "@bob", "checking in", draftQuery{asJSON: true}); err != nil {
		t.Fatal(err)
	}
	var got draftJSON
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.ChatID != 42 || got.TopicID != 0 || got.Cleared {
		t.Fatalf("set json = %+v", got)
	}
	if len(acc.savedDrafts) != 1 || acc.savedDrafts[0].text != "checking in" {
		t.Fatalf("savedDrafts = %+v", acc.savedDrafts)
	}

	// Clearing a forum without --topic defaults to GeneralTopicID.
	out.Reset()
	if err := saveDraft(shortCtx(t), acc, &out, "@gophers", "", draftQuery{asJSON: true}); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.ChatID != 300 || got.TopicID != domain.GeneralTopicID || !got.Cleared {
		t.Fatalf("clear forum json = %+v", got)
	}
	if acc.savedDrafts[1].h != (domain.HistoryKey{ChatID: 300, TopicID: domain.GeneralTopicID}) || acc.savedDrafts[1].text != "" {
		t.Fatalf("savedDrafts[1] = %+v", acc.savedDrafts[1])
	}
}

func TestDraft_NonForumWithTopicExitsFiveAndSanitizesTitle(t *testing.T) {
	acc := newFakeAccount()
	acc.byName = map[core.ChatName]project.ChatRow{{ID: 42}: {ID: 42, Title: "\x1b[31mBob\u202e"}}

	err := saveDraft(shortCtx(t), acc, &bytes.Buffer{}, "42", "hi", draftQuery{topic: 5})
	var ec *exitCode
	if !errors.As(err, &ec) || ec.code != notFoundCode {
		t.Fatalf("got %v, want exit %d", err, notFoundCode)
	}
	if strings.Contains(ec.msg, "\x1b") || strings.Contains(ec.msg, "\u202e") || !strings.Contains(ec.msg, "Bob") {
		t.Fatalf("unsanitized error: %q", ec.msg)
	}
}
