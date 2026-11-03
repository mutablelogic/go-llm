package httpclient

import (
	"time"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	types "github.com/mutablelogic/go-server/pkg/types"
)

////////////////////////////////////////////////////////////////////////////////
// RESPONSE TYPES

// Article is a news article
type Article struct {
	Source      Source    `json:"source"`
	Title       string    `json:"title"`
	Author      string    `json:"author,omitempty"`
	Description string    `json:"description,omitempty"`
	Url         string    `json:"url,omitempty"`
	ImageUrl    string    `json:"urlToImage,omitempty"`
	PublishedAt time.Time `json:"publishedAt"`
	Content     string    `json:"content,omitempty"`
}

// Source is a news source, such as a newspaper or blog
type Source struct {
	Id          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Url         string `json:"url,omitempty"`
	Category    string `json:"category,omitempty"`
	Language    string `json:"language,omitempty"`
	Country     string `json:"country,omitempty"`
}

// status is the status of a response, which has an error code and message
// when it's not "ok"
type status struct {
	Status  string `json:"status"`
	Code    string `json:"code,omitempty"`
	Message string `json:"message,omitempty"`
}

type respArticles struct {
	status
	TotalResults int       `json:"totalResults"`
	Articles     []Article `json:"articles"`
}

type respSources struct {
	status
	Sources []Source `json:"sources"`
}

////////////////////////////////////////////////////////////////////////////////
// STRINGIFY

func (a Article) String() string {
	return types.Stringify(a)
}

func (s Source) String() string {
	return types.Stringify(s)
}

////////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// err returns an error when the status of the response isn't "ok"
func (s status) err() error {
	if s.Status == "ok" {
		return nil
	}
	return llm.ErrBadParameter.Withf("%s: %s", s.Code, s.Message)
}
