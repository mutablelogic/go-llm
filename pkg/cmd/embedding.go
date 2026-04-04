package cmd

import (
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"

	// Packages
	otel "github.com/mutablelogic/go-client/pkg/otel"
	httpclient "github.com/mutablelogic/go-llm/pkg/httpclient-new"
	schema "github.com/mutablelogic/go-llm/pkg/schema"
	server "github.com/mutablelogic/go-server"
	types "github.com/mutablelogic/go-server/pkg/types"
	attribute "go.opentelemetry.io/otel/attribute"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

type EmbeddingCommands struct {
	Embedding EmbeddingCommand `cmd:"" name:"embedding" help:"Generate embedding vectors from input text or the first column of a CSV file." group:"RESPOND"`
}

type EmbeddingCommand struct {
	schema.EmbeddingRequest `embed:""`
	CSV                     string `name:"csv" type:"file" placeholder:"FILE" help:"Path to input CSV file; the first column is embedded and all other columns are ignored" optional:""`
	Out                     string `name:"out" type:"file" placeholder:"FILE" help:"Path to output CSV file (defaults to stdout)" optional:""`
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

func (cmd *EmbeddingCommand) Run(ctx server.Cmd) (err error) {
	if cmd.Model == "" {
		cmd.Model = ctx.GetString("embedding_model")
	}
	if cmd.Provider == "" {
		cmd.Provider = ctx.GetString("embedding_provider")
	}
	if cmd.Model == "" {
		return fmt.Errorf("embedding model is required (set with --model or store a default)")
	}
	if err := ctx.Set("embedding_model", cmd.Model); err != nil {
		return err
	}
	if cmd.Provider != "" {
		if err := ctx.Set("embedding_provider", cmd.Provider); err != nil {
			return err
		}
	}

	input, err := cmd.input()
	if err != nil {
		return err
	}

	req := cmd.EmbeddingRequest
	req.Input = input

	return WithClient(ctx, func(client *httpclient.Client, _ string) error {
		outputPath := cmd.outputPath()
		parent, endSpan := otel.StartSpan(ctx.Tracer(), ctx.Context(), "EmbeddingCommand",
			attribute.String("request", types.Stringify(req)),
			attribute.String("csv", cmd.CSV),
			attribute.String("out", outputPath),
		)
		defer func() { endSpan(err) }()

		response, err := client.Embedding(parent, req)
		if err != nil {
			return err
		}
		if outputPath == "" {
			if err := writeEmbeddingCSVToWriter(os.Stdout, input, response.Output); err != nil {
				return err
			}
		} else {
			if err := writeEmbeddingCSV(outputPath, input, response.Output); err != nil {
				return err
			}
		}

		if ctx.IsDebug() {
			fmt.Println(response)
		}
		if outputPath != "" {
			fmt.Printf("Wrote %d embeddings to %s\n", len(response.Output), outputPath)
		}
		return nil
	})
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

func (cmd EmbeddingCommand) input() ([]string, error) {
	if cmd.CSV != "" && len(cmd.Input) > 0 {
		return nil, fmt.Errorf("--csv and input arguments are mutually exclusive")
	}
	if cmd.CSV != "" {
		input, err := readEmbeddingCSV(cmd.CSV)
		if err != nil {
			return nil, err
		}
		if len(input) == 0 {
			return nil, fmt.Errorf("input CSV %q does not contain any rows", cmd.CSV)
		}
		return input, nil
	}
	if len(cmd.Input) == 0 {
		return nil, fmt.Errorf("either input text or --csv is required")
	}
	return cmd.Input, nil
}

func (cmd EmbeddingCommand) outputPath() string {
	if cmd.Out != "" {
		return cmd.Out
	}
	return ""
}

func readEmbeddingCSV(path string) ([]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	reader := csv.NewReader(file)
	reader.FieldsPerRecord = -1
	result := make([]string, 0, 128)
	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		if len(record) == 0 {
			continue
		}
		result = append(result, record[0])
	}

	return result, nil
}

func writeEmbeddingCSV(path string, input []string, output [][]float64) error {
	if len(input) != len(output) {
		return fmt.Errorf("embedding row count mismatch: got %d rows for %d inputs", len(output), len(input))
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	return writeEmbeddingCSVToWriter(file, input, output)
}

func writeEmbeddingCSVToWriter(w io.Writer, input []string, output [][]float64) error {
	if len(input) != len(output) {
		return fmt.Errorf("embedding row count mismatch: got %d rows for %d inputs", len(output), len(input))
	}

	writer := csv.NewWriter(w)
	for i, text := range input {
		row := make([]string, 1, len(output[i])+1)
		row[0] = text
		for _, value := range output[i] {
			row = append(row, strconv.FormatFloat(value, 'g', -1, 64))
		}
		if err := writer.Write(row); err != nil {
			return err
		}
	}
	writer.Flush()
	return writer.Error()
}
