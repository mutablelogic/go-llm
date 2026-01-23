package main

import (
	"context"
	"fmt"
	"os"

	"github.com/mutablelogic/go-llm"
	"github.com/mutablelogic/go-llm/pkg/openai"
)

const (
	model   = "dall-e-3"
	size    = "1024x1024"
	quality = "standard"
	style   = "vivid"
)

func main() {
	// Create a new OpenAI agent
	agent, err := openai.New(os.Getenv("OPENAI_API_KEY"))
	if err != nil {
		panic(err)
	}

	// Check args
	if len(os.Args) != 2 {
		fmt.Println("Usage: image_generate <caption>")
		os.Exit(-1)
	}

	// Get a model
	model := agent.Model(context.TODO(), model)

	// Get image
	images, err := model.Completion(
		context.TODO(),
		os.Args[1],
		llm.WithFormat("image"),
		llm.WithQuality(quality),
		llm.WithSize(size),
		llm.WithStyle(style),
	)
	if err != nil {
		panic(err)
	}

	// Write out image(s)
	for i := 0; i < images.Num(); i++ {
		attachment := images.Attachment(i)
		if attachment == nil || attachment.Filename() == "" {
			continue
		}

		// Create a file
		f, err := os.Create(attachment.Filename())
		if err != nil {
			panic(err)
		}
		defer f.Close()

		// Write the image
		if _, err := f.Write(attachment.Data()); err != nil {
			panic(err)
		} else {
			fmt.Printf("%q written to %s\n", attachment.Caption(), attachment.Filename())
		}
	}

}
