// Package notifier provides a stand-in notification channel for the demo:
// there's no push/SMS/email provider wired up, just a log line, so the
// saga's terminal outcomes are visible without external dependencies.
package notifier

import (
	"context"
	"log"
)

type Log struct{}

func NewLog() *Log { return &Log{} }

func (l *Log) Send(_ context.Context, bookingID, message string) error {
	log.Printf("notify booking=%s: %s", bookingID, message)
	return nil
}
