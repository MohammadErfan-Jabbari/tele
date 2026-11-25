package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/sorokin-vladimir/tele/internal/core"
	"github.com/sorokin-vladimir/tele/internal/domain"
	"github.com/sorokin-vladimir/tele/internal/sanitize"
	"github.com/sorokin-vladimir/tele/internal/telerr"
)

const defaultMediaTimeout = 5 * time.Minute

type mediaSaveQuery struct {
	topic   int
	dir     string
	slot    string
	timeout time.Duration
	asJSON  bool
}

type mediaSavedJSON struct {
	ChatID    int64  `json:"chat_id"`
	MessageID int    `json:"message_id"`
	Slot      string `json:"slot"`
	Path      string `json:"path"`
	Size      int64  `json:"size"`
}

// newMediaCmd is `tele media`: download media attachments to local files.
func newMediaCmd(o *options) *cobra.Command {
	root := &cobra.Command{
		Use:   "media",
		Short: "Download media from a message",
	}
	var q mediaSaveQuery
	save := &cobra.Command{
		Use:   "save CHAT MSG_ID",
		Short: "Save a message's media attachment into a directory",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if _, err := parseChatName(args[0]); err != nil {
				return err
			}
			msgID, err := strconv.Atoi(args[1])
			if err != nil || msgID <= 0 {
				return &exitCode{code: usageCode, msg: "MSG_ID must be a positive message id."}
			}
			if strings.TrimSpace(q.dir) == "" {
				return &exitCode{code: usageCode, msg: "--dir is required."}
			}
			if q.timeout <= 0 {
				return &exitCode{code: usageCode, msg: "--timeout is more than nothing, such as 1m or 5m."}
			}
			if q.slot != "" {
				if _, ok := parseMediaSlot(q.slot); !ok {
					return &exitCode{code: usageCode, msg: "--slot must be one of photo, photo_thumb, document, doc_thumb."}
				}
			}
			return askAccount(cmd.Context(), *o, q.timeout, func(ctx context.Context, a heldAccount) error {
				return saveMedia(ctx, a, cmd.OutOrStdout(), args[0], msgID, q)
			})
		},
	}
	f := save.Flags()
	f.IntVar(&q.topic, "topic", 0, "the topic of a forum containing the message, by id")
	f.StringVar(&q.dir, "dir", "", "directory to save the downloaded file into (required)")
	f.StringVar(&q.slot, "slot", "", "media slot: photo, photo_thumb, document, or doc_thumb (auto-detected when omitted)")
	f.DurationVar(&q.timeout, "timeout", defaultMediaTimeout, "how long to wait for the download")
	f.BoolVar(&q.asJSON, "json", false, "print the result as JSON")
	root.AddCommand(save)
	return root
}

func parseMediaSlot(raw string) (domain.MediaSlot, bool) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "photo", "photo_full":
		return domain.PhotoFull, true
	case "photo_thumb":
		return domain.PhotoThumb, true
	case "document", "doc", "doc_full":
		return domain.DocFull, true
	case "doc_thumb":
		return domain.DocThumb, true
	default:
		return 0, false
	}
}

func saveMedia(ctx context.Context, acc accountSurface, out io.Writer, chat string, msgID int, q mediaSaveQuery) error {
	row, err := resolveChat(ctx, acc, chat)
	if err != nil {
		return err
	}
	title := sanitize.Name(row.Title)
	switch {
	case row.IsForum && q.topic == 0:
		return &exitCode{code: notFoundCode, msg: fmt.Sprintf(
			"%s is a forum: its messages are its topics'. Name one with --topic; `tele topics %s` lists them.", title, chat)}
	case !row.IsForum && q.topic != 0:
		return &exitCode{code: notFoundCode, msg: fmt.Sprintf("%s is not a forum: it has no topics.", title)}
	}
	h := domain.HistoryKey{ChatID: row.ID, TopicID: q.topic}
	msgs, err := acc.HistoryAround(ctx, h, msgID, 1)
	if core.IsNoMessage(err) {
		return &exitCode{code: notFoundCode, msg: fmt.Sprintf("No message %d in %s.", msgID, title)}
	}
	if telerr.Of(err) == telerr.PeerNotFound || telerr.Of(err) == telerr.NotFound {
		if q.topic != 0 {
			return &exitCode{code: notFoundCode, msg: fmt.Sprintf("No topic %d in %s.", q.topic, title)}
		}
		return &exitCode{code: notFoundCode, msg: fmt.Sprintf("No message %d in %s.", msgID, title)}
	}
	if err != nil {
		return err
	}
	var target *domain.Message
	for i := range msgs {
		if msgs[i].ID == msgID {
			target = &msgs[i]
			break
		}
	}
	if target == nil {
		return &exitCode{code: notFoundCode, msg: fmt.Sprintf("No message %d in %s.", msgID, title)}
	}

	var slot domain.MediaSlot
	if q.slot != "" {
		parsed, ok := parseMediaSlot(q.slot)
		if !ok {
			return &exitCode{code: usageCode, msg: "--slot must be one of photo, photo_thumb, document, doc_thumb."}
		}
		slot = parsed
	} else {
		detected, ok := detectMediaSlot(*target)
		if !ok {
			return &exitCode{code: notFoundCode, msg: fmt.Sprintf("Message %d in %s has no downloadable media.", msgID, title)}
		}
		slot = detected
	}

	absDir, err := filepath.Abs(strings.TrimSpace(q.dir))
	if err != nil {
		return fmt.Errorf("resolving --dir: %w", err)
	}
	savedPath, err := acc.SaveMedia(ctx, row.ID, msgID, slot, absDir, "")
	if telerr.Of(err) == telerr.NotFound {
		return &exitCode{code: notFoundCode, msg: fmt.Sprintf("Message %d in %s has no %s media.", msgID, title, slot.String())}
	}
	if err != nil {
		return err
	}
	canonDir, err := canonicalPath(absDir)
	if err != nil {
		return fmt.Errorf("resolving destination directory: %w", err)
	}
	canonSaved, err := canonicalPath(savedPath)
	if err != nil {
		return fmt.Errorf("resolving saved path: %w", err)
	}
	rel, err := filepath.Rel(canonDir, canonSaved)
	if err != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return fmt.Errorf("saved file path escaped destination directory")
	}
	var size int64
	if info, statErr := os.Stat(canonSaved); statErr == nil {
		size = info.Size()
	}
	if q.asJSON {
		return json.NewEncoder(out).Encode(mediaSavedJSON{
			ChatID:    row.ID,
			MessageID: msgID,
			Slot:      slot.String(),
			Path:      canonSaved,
			Size:      size,
		})
	}
	_, err = fmt.Fprintln(out, canonSaved)
	return err
}

// canonicalPath resolves symlinks on the longest existing prefix of p so path
// containment holds across symlinked directories (such as /tmp -> /private/tmp)
// and catches symlinks inside --dir that point outside it.
func canonicalPath(p string) (string, error) {
	abs, err := filepath.Abs(p)
	if err != nil {
		return "", err
	}
	resolved, err := filepath.EvalSymlinks(abs)
	if err == nil {
		return resolved, nil
	}
	if !os.IsNotExist(err) {
		return "", err
	}
	dir := filepath.Dir(abs)
	if dir == abs {
		return abs, nil
	}
	parent, err := canonicalPath(dir)
	if err != nil {
		return "", err
	}
	return filepath.Join(parent, filepath.Base(abs)), nil
}

func detectMediaSlot(m domain.Message) (domain.MediaSlot, bool) {
	if m.Photo != nil {
		return domain.PhotoFull, true
	}
	if m.Document != nil {
		return domain.DocFull, true
	}
	if m.Media == nil {
		return 0, false
	}
	switch m.Media.Kind {
	case domain.MediaPhoto:
		return domain.PhotoFull, true
	case domain.MediaVideo, domain.MediaVideoNote, domain.MediaVoice, domain.MediaAudio, domain.MediaSticker, domain.MediaGIF, domain.MediaFile:
		return domain.DocFull, true
	default:
		return 0, false
	}
}
