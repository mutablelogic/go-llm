package schema

import "strings"

///////////////////////////////////////////////////////////////////////////////
// PROVIDER TABLE

func (t Provider) Header() []string {
	return []string{"NAME", "PROVIDER", "URL", "ENABLED", "INCLUDE", "EXCLUDE", "CREATED AT", "MODIFIED AT"}
}

func (t Provider) Width(i int) int {
	switch i {
	case 0:
		return 20
	case 1:
		return 12
	case 2:
		return 32
	case 3:
		return 8
	case 4, 5:
		return 24
	case 6, 7:
		return 19
	}
	return 0
}

func (t Provider) Cell(i int) string {
	switch i {
	case 0:
		return t.Name
	case 1:
		return t.Provider
	case 2:
		if t.URL != nil {
			return *t.URL
		}
	case 3:
		if t.Enabled != nil && *t.Enabled {
			return "true"
		}
		return "false"
	case 4:
		return strings.Join(t.Include, ", ")
	case 5:
		return strings.Join(t.Exclude, ", ")
	case 6:
		if !t.CreatedAt.IsZero() {
			return t.CreatedAt.Format("2006-01-02 15:04:05")
		}
	case 7:
		if t.ModifiedAt != nil {
			return t.ModifiedAt.Format("2006-01-02 15:04:05")
		}
	}
	return ""
}

///////////////////////////////////////////////////////////////////////////////
// MODELS TABLE

func (t Model) Header() []string {
	return []string{"NAME", "PROVIDER", "DESCRIPTION", "CREATED AT", "ALIASES"}
}

func (t Model) Width(i int) int {
	switch i {
	case 0:
		return 24
	case 1:
		return 12
	case 2:
		return 40
	case 3:
		return 19
	case 4:
		return 24
	}
	return 0
}

func (t Model) Cell(i int) string {
	switch i {
	case 0:
		return t.Name
	case 1:
		return t.OwnedBy
	case 2:
		return t.Description
	case 3:
		if !t.Created.IsZero() {
			return t.Created.Format("2006-01-02 15:04:05")
		}
	case 4:
		return strings.Join(t.Aliases, ", ")
	}
	return ""
}
