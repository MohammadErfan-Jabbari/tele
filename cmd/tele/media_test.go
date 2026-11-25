package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sorokin-vladimir/tele/internal/core"
	"github.com/sorokin-vladimir/tele/internal/core/project"
	"github.com/sorokin-vladimir/tele/internal/domain"
	"github.com/sorokin-vladimir/tele/internal/telerr"
)

type saveMediaAsk struct {
	chatID  int64
	msgID   int
	slot    domain.MediaSlot
	destDir string
	ref     string
}

func (f *fakeAccount) SaveMedia(_ context.Context, chatID int64, msgID int, slot domain.MediaSlot, destDir, ref string) (string, error) {
	f.savedMedia = append(f.savedMedia, saveMediaAsk{chatID, msgID, slot, destDir, ref})
	if f.saveErr != nil {
		return "", f.saveErr
	}
	return f.savePath, nil
}

func TestMediaSave_AutoDetectsPhotoAndDocumentAndPrintsJSON(t *testing.T) {
	dir := t.TempDir()
	saved := filepath.Join(dir, "report.pdf")
	if err := os.WriteFile(saved, []byte("pdf-bytes"), 0o600); err != nil {
		t.Fatal(err)
	}

	acc := newFakeAccount()
	acc.byName = map[core.ChatName]project.ChatRow{{Username: "bob"}: {ID: 42, Title: "Bob"}}
	acc.history = []domain.Message{
		{ID: 100, Media: &domain.MediaRef{Kind: domain.MediaFile, FileName: "report.pdf"}},
	}
	acc.savePath = saved

	var out bytes.Buffer
	err := saveMedia(shortCtx(t), acc, &out, "@bob", 100, mediaSaveQuery{dir: dir, asJSON: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(acc.savedMedia) != 1 || acc.savedMedia[0].slot != domain.DocFull || acc.savedMedia[0].msgID != 100 {
		t.Fatalf("savedMedia = %+v, want DocFull for msg 100", acc.savedMedia)
	}
	var got mediaSavedJSON
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	wantPath, err := canonicalPath(saved)
	if err != nil {
		t.Fatal(err)
	}
	if got.ChatID != 42 || got.MessageID != 100 || got.Slot != "doc_full" || got.Path != wantPath || got.Size != 9 {
		t.Fatalf("got json %+v, want path %q", got, wantPath)
	}
}

func TestMediaSave_AutoDetectsPhotoAndExplicitSlotOverride(t *testing.T) {
	dir := t.TempDir()
	saved := filepath.Join(dir, "photo_1.jpg")
	if err := os.WriteFile(saved, []byte("jpg"), 0o600); err != nil {
		t.Fatal(err)
	}

	acc := newFakeAccount()
	acc.byName = map[core.ChatName]project.ChatRow{{ID: 42}: {ID: 42, Title: "Bob"}}
	acc.history = []domain.Message{
		{ID: 55, Media: &domain.MediaRef{Kind: domain.MediaPhoto}},
	}
	acc.savePath = saved

	var out bytes.Buffer
	if err := saveMedia(shortCtx(t), acc, &out, "42", 55, mediaSaveQuery{dir: dir}); err != nil {
		t.Fatal(err)
	}
	if acc.savedMedia[0].slot != domain.PhotoFull {
		t.Fatalf("slot = %v, want PhotoFull", acc.savedMedia[0].slot)
	}
	wantPath, err := canonicalPath(saved)
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(out.String()) != wantPath {
		t.Fatalf("out = %q, want %q", out.String(), wantPath)
	}

	out.Reset()
	if err := saveMedia(shortCtx(t), acc, &out, "42", 55, mediaSaveQuery{dir: dir, slot: "photo_thumb"}); err != nil {
		t.Fatal(err)
	}
	if acc.savedMedia[1].slot != domain.PhotoThumb {
		t.Fatalf("slot = %v, want PhotoThumb", acc.savedMedia[1].slot)
	}
}

func TestMediaSave_RejectsEscapedPathAndMissingMediaAndForumWithoutTopic(t *testing.T) {
	dir := t.TempDir()
	acc := newFakeAccount()
	acc.byName = map[core.ChatName]project.ChatRow{
		{ID: 42}:  {ID: 42, Title: "\x1b[31mBob\u202e"},
		{ID: 300}: {ID: 300, Title: "Forum", IsForum: true},
	}

	// Forum without --topic exits 5.
	err := saveMedia(shortCtx(t), acc, &bytes.Buffer{}, "300", 10, mediaSaveQuery{dir: dir})
	var ec *exitCode
	if !errors.As(err, &ec) || ec.code != notFoundCode {
		t.Fatalf("forum without topic: got %v, want exit %d", err, notFoundCode)
	}

	// Message with no downloadable media exits 5 and sanitizes title.
	acc.history = []domain.Message{{ID: 10, Text: "plain text"}}
	err = saveMedia(shortCtx(t), acc, &bytes.Buffer{}, "42", 10, mediaSaveQuery{dir: dir})
	if !errors.As(err, &ec) || ec.code != notFoundCode || strings.Contains(ec.msg, "\x1b") || strings.Contains(ec.msg, "\u202e") {
		t.Fatalf("no media: got %v, want sanitized exit %d", err, notFoundCode)
	}

	// Escaped path outside --dir is refused.
	acc.history = []domain.Message{{ID: 11, Media: &domain.MediaRef{Kind: domain.MediaFile}}}
	acc.savePath = filepath.Join(filepath.Dir(dir), "outside.bin")
	err = saveMedia(shortCtx(t), acc, &bytes.Buffer{}, "42", 11, mediaSaveQuery{dir: dir})
	if err == nil || !strings.Contains(err.Error(), "escaped") {
		t.Fatalf("escaped path: got %v, want escape error", err)
	}

	// Symlink inside --dir pointing outside --dir is also refused via canonical containment.
	outsideFile := filepath.Join(t.TempDir(), "secret.txt")
	if err := os.WriteFile(outsideFile, []byte("secret"), 0o600); err != nil {
		t.Fatal(err)
	}
	linkPath := filepath.Join(dir, "link.txt")
	if err := os.Symlink(outsideFile, linkPath); err != nil {
		t.Fatal(err)
	}
	acc.savePath = linkPath
	err = saveMedia(shortCtx(t), acc, &bytes.Buffer{}, "42", 11, mediaSaveQuery{dir: dir})
	if err == nil || !strings.Contains(err.Error(), "escaped") {
		t.Fatalf("symlink escape: got %v, want escape error", err)
	}

	// Missing media slot from host exits 5.
	acc.savePath = ""
	acc.saveErr = &telerr.Error{Kind: telerr.NotFound, Op: "SaveMedia"}
	err = saveMedia(shortCtx(t), acc, &bytes.Buffer{}, "42", 11, mediaSaveQuery{dir: dir})
	if !errors.As(err, &ec) || ec.code != notFoundCode {
		t.Fatalf("host not found: got %v, want exit %d", err, notFoundCode)
	}
}
