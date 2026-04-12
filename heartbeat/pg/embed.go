package pg

import _ "embed"

//go:embed objects.sql
var Objects string

//go:embed queries.sql
var Queries string
