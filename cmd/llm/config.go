package main

import (
	"encoding/json"
	"os"
	"path/filepath"
)

////////////////////////////////////////////////////////////////////////////////
// TYPES

type Config struct {
	// Default models for each type
	ImageModel      string `json:"image_model"`
	AudioModel      string `json:"audio_model"`
	TextModel       string `json:"text_model"`
	ChatModel       string `json:"chat_model"`
	EmbeddingsModel string `json:"embeddings_model"`

	// Path to the config file
	path string
}

type Type int

//////////////////////////////////////////////////////////////////
// GLOBALS

const (
	// The name of the config file
	configFile = "config.json"
)

const (
	ImageType Type = iota
	AudioType
	TextType
	ChatType
	EmbeddingsType
)

//////////////////////////////////////////////////////////////////
// LIFECYCLE

// Create a new state object with the given name
func NewConfig(name string) (*Config, error) {
	// Load the state from the file, or return a new empty state
	path, err := os.UserConfigDir()
	if err != nil {
		return nil, err
	}

	// Append the name of the application to the path
	if name != "" {
		path = filepath.Join(path, name)
	}

	// Create the directory if it doesn't exist
	if err := os.MkdirAll(path, 0700); err != nil {
		return nil, err
	}

	// The state to return
	var config Config
	config.path = filepath.Join(path, configFile)

	// Load the state from the file, ignore any errors
	_ = config.Load()

	// Return success
	return &config, nil
}

// Release resources
func (s *Config) Close() error {
	return s.Save()
}

//////////////////////////////////////////////////////////////////
// PUBLIC METHODS

func (s *Config) ModelFor(typ Type) string {
	switch typ {
	case ImageType:
		return s.ImageModel
	case AudioType:
		return s.AudioModel
	case TextType:
		return s.TextModel
	case ChatType:
		return s.ChatModel
	case EmbeddingsType:
		return s.EmbeddingsModel
	}
	return ""
}

func (s *Config) SetModelFor(typ Type, model string) {
	switch typ {
	case ImageType:
		s.ImageModel = model
	case AudioType:
		s.AudioModel = model
	case TextType:
		s.TextModel = model
	case ChatType:
		s.ChatModel = model
	case EmbeddingsType:
		s.EmbeddingsModel = model
	}
}

//////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// Load state as JSON
func (s *Config) Load() error {
	// Open the file
	file, err := os.Open(s.path)
	if err != nil {
		return nil
	}
	defer file.Close()

	// Decode the JSON
	if err := json.NewDecoder(file).Decode(s); err != nil {
		return err
	}

	// Return success
	return nil
}

// Save state as JSON
func (s *Config) Save() error {
	// Open the file
	file, err := os.Create(s.path)
	if err != nil {
		return err
	}
	defer file.Close()

	// Encode the JSON
	return json.NewEncoder(file).Encode(s)
}
