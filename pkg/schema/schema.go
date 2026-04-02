package schema

import (
	_ "embed"
)

///////////////////////////////////////////////////////////////////////////////
// GLOBALS

//go:embed objects.sql
var Objects string

//go:embed queries.sql
var Queries string

const (
	DefaultSchema        = "llm"
	DefaultAuthSchema    = "auth"
	DefaultNotifyChannel = "channel_notify"
)
