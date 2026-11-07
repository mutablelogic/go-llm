// Package schema has the channels of messaging: chats, such as on Telegram,
// which are attached to sessions, so the person can talk to their assistant
// there
package schema

import (
	"time"
	"uuid"

	// Packages
	types "github.com/mutablelogic/go-server/pkg/types"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// ChannelMeta is a channel's chat, and the session it's attached to
type ChannelMeta struct {
	Session  uuid.UUID `json:"session" help:"The session the channel is attached to"`
	Provider string    `json:"provider" help:"The messaging provider, such as telegram" example:"telegram"`
	Address  string    `json:"address" help:"The chat on the provider, such as a Telegram chat's identifier" example:"123456789"`
	Title    string    `json:"title,omitempty" help:"The chat's name, such as the person's, or a group's" example:"David"`
}

// Channel is a chat, such as on Telegram, attached to a session, so the
// person's messages there are sent to the session, and its replies are
// sent there
type Channel struct {
	ID uuid.UUID `json:"id" help:"The channel's identifier"`
	ChannelMeta
	Created time.Time `json:"created" help:"When the chat was connected"`
}

///////////////////////////////////////////////////////////////////////////////
// STRINGIFY

func (c ChannelMeta) String() string {
	return types.Stringify(c)
}

func (c Channel) String() string {
	return types.Stringify(c)
}
