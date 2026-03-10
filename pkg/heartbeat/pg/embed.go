package pg

import _ "embed"

//go:embed objects.sql
var objects string

//go:embed queries.sql
var queries string
