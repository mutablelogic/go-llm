package telegram

import (
	"context"
	"fmt"

	// Packages
	telegram "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

/////////////////////////////////////////////////////////////////////
// TYPES

type t struct {
	*telegram.BotAPI
}

/////////////////////////////////////////////////////////////////////
// LIFECYCLE

func NewTelegram(token string) (*t, error) {
	bot, err := telegram.NewBotAPI(token)
	if err != nil {
		return nil, err
	}

	// Create a new telegram instance
	telegram := &t{bot}

	// Return the instance
	return telegram, nil
}

/////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

func (t *t) Run(ctx context.Context) error {
	updates := t.GetUpdatesChan(telegram.NewUpdate(0))
FOR_LOOP:
	for {
		select {
		case <-ctx.Done():
			break FOR_LOOP
		case evt := <-updates:
			if evt.Message != nil && !evt.Message.IsCommand() {
				t.handleMessage(evt.Message)
			}
		}
	}

	// Return success
	return nil
}

/////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

func (t *t) handleMessage(update *telegram.Message) {
	fmt.Println("Received message from", update.From.UserName)
	fmt.Println(" => ", update.Text)
}
