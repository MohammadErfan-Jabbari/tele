package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/sorokin-vladimir/tele/internal/domain"
	"github.com/sorokin-vladimir/tele/internal/readonly"
	"github.com/sorokin-vladimir/tele/internal/sanitize"
)

type draftQuery struct {
	topic  int
	asJSON bool
}

type draftJSON struct {
	ChatID  int64 `json:"chat_id"`
	TopicID int   `json:"topic_id,omitempty"`
	Cleared bool  `json:"cleared"`
}

// newDraftCmd is `tele draft`: stage or clear a Telegram draft without sending.
func newDraftCmd(o *options) *cobra.Command {
	root := &cobra.Command{
		Use:   "draft",
		Short: "Set or clear an unsent draft in a chat",
	}

	var setQ draftQuery
	setCmd := &cobra.Command{
		Use:   "set CHAT [TEXT...]",
		Short: "Save an unsent draft message in a chat",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if _, err := parseChatName(args[0]); err != nil {
				return err
			}
			if readonly.Active() {
				return &exitCode{code: commandsOffCode, msg: "Not allowed: TELE_READ_ONLY is enabled in the process environment."}
			}
			text, err := composeText(args[1:], cmd.InOrStdin())
			if err != nil {
				return err
			}
			return askAccount(cmd.Context(), *o, readyTimeout, func(ctx context.Context, a heldAccount) error {
				return saveDraft(ctx, a, cmd.OutOrStdout(), args[0], text, setQ)
			})
		},
	}
	setCmd.Flags().IntVar(&setQ.topic, "topic", 0, "the topic of a forum to save the draft in, by id; General when left out")
	setCmd.Flags().BoolVar(&setQ.asJSON, "json", false, "print the result as JSON")

	var clearQ draftQuery
	clearCmd := &cobra.Command{
		Use:   "clear CHAT",
		Short: "Clear the unsent draft in a chat",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if _, err := parseChatName(args[0]); err != nil {
				return err
			}
			if readonly.Active() {
				return &exitCode{code: commandsOffCode, msg: "Not allowed: TELE_READ_ONLY is enabled in the process environment."}
			}
			return askAccount(cmd.Context(), *o, readyTimeout, func(ctx context.Context, a heldAccount) error {
				return saveDraft(ctx, a, cmd.OutOrStdout(), args[0], "", clearQ)
			})
		},
	}
	clearCmd.Flags().IntVar(&clearQ.topic, "topic", 0, "the topic of a forum to clear the draft in, by id; General when left out")
	clearCmd.Flags().BoolVar(&clearQ.asJSON, "json", false, "print the result as JSON")

	root.AddCommand(setCmd, clearCmd)
	return root
}

func saveDraft(ctx context.Context, acc accountSurface, out io.Writer, chat, text string, q draftQuery) error {
	row, err := resolveChat(ctx, acc, chat)
	if err != nil {
		return err
	}
	title := sanitize.Name(row.Title)
	topic := q.topic
	switch {
	case !row.IsForum && topic != 0:
		return &exitCode{code: notFoundCode, msg: fmt.Sprintf("%s is not a forum: it has no topics.", title)}
	case row.IsForum && topic == 0:
		topic = domain.GeneralTopicID
	}
	if err := acc.SaveDraft(ctx, domain.HistoryKey{ChatID: row.ID, TopicID: topic}, text); err != nil {
		return err
	}
	cleared := text == ""
	if q.asJSON {
		return json.NewEncoder(out).Encode(draftJSON{
			ChatID:  row.ID,
			TopicID: topic,
			Cleared: cleared,
		})
	}
	if cleared {
		_, err = fmt.Fprintln(out, "draft cleared")
		return err
	}
	_, err = fmt.Fprintln(out, "draft saved")
	return err
}
