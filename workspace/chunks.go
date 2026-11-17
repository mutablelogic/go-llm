package workspace

import (
	"context"
	"errors"
	"io"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	client "github.com/mutablelogic/go-llm/mcp/client"
	schema "github.com/mutablelogic/go-llm/workspace/schema"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// chunked is a resource's content, which an MCP server which is a
// workspace returns a chunk at a time, as it's read, from the first chunk,
// whose size is -1, when the server doesn't know it, when it's read until
// a chunk is empty
type chunked struct {
	client              *client.Client
	uri, name, mimeType string
	size                int64
	chunks              bool // Whether the server returns chunks
	first               []byte
}

// chunkReader reads a chunked resource's content, fetching each chunk as
// it's read, and seeks in it
type chunkReader struct {
	ctx      context.Context
	resource *chunked
	offset   int64  // of the reader in the content
	start    int64  // of the chunk in the content
	chunk    []byte // the chunk which was fetched last
}

var _ llm.Resource = (*chunked)(nil)
var _ io.ReadSeeker = (*chunkReader)(nil)

///////////////////////////////////////////////////////////////////////////////
// GLOBALS

// ChunkSize is the most bytes of a resource's content which are read from
// an MCP server at once, which is less than its server's MaxChunk, so its
// JSON-RPC frame, in base64, is less than the SDK reads
const ChunkSize = 4 * 1024 * 1024

///////////////////////////////////////////////////////////////////////////////
// RESOURCE

func (c *chunked) URI() string          { return c.uri }
func (c *chunked) Name() string         { return c.name }
func (c *chunked) Description() string  { return "" }
func (c *chunked) Type() string         { return c.mimeType }
func (c *chunked) ContentLength() int64 { return c.size }

// Read returns a function which reads the content, from the first chunk,
// fetching the others as they're read, with the function's context
func (c *chunked) Read(context.Context) (llm.ReadFunc, error) {
	return func(ctx context.Context, fn func(io.Reader) error) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		return fn(&chunkReader{ctx: ctx, resource: c, chunk: c.first})
	}, nil
}

///////////////////////////////////////////////////////////////////////////////
// READER

// Read reads from the chunk which has the offset, fetching it when it
// isn't the last fetched
func (r *chunkReader) Read(p []byte) (int, error) {
	size := r.resource.size
	if len(p) == 0 {
		return 0, nil
	} else if size >= 0 && r.offset >= size {
		return 0, io.EOF
	}
	if r.offset < r.start || r.offset >= r.start+int64(len(r.chunk)) {
		if err := r.fetch(); err != nil {
			return 0, err
		}
	}
	// No more than its size, as it was, should it have grown since
	chunk := r.chunk[r.offset-r.start:]
	if size >= 0 {
		chunk = chunk[:min(int64(len(chunk)), size-r.offset)]
	}
	n := copy(p, chunk)
	r.offset += int64(n)
	return n, nil
}

// Seek sets the offset of the next read, which fetches its chunk when it
// isn't the last fetched
func (r *chunkReader) Seek(offset int64, whence int) (int64, error) {
	switch whence {
	case io.SeekStart:
	case io.SeekCurrent:
		offset += r.offset
	case io.SeekEnd:
		if r.resource.size < 0 {
			return 0, errors.New("seek: size isn't known")
		}
		offset += r.resource.size
	default:
		return 0, errors.New("seek: invalid whence")
	}
	if offset < 0 {
		return 0, errors.New("seek: negative position")
	}
	r.offset = offset
	return offset, nil
}

// fetch fetches the chunk from the offset. Returns EOF when it's empty,
// and its size isn't known, and ErrUnexpectedEOF when it's empty, before
// the end of the content, as the content changed.
func (r *chunkReader) fetch() error {
	data, _, err := readChunk(r.ctx, r.resource.client, r.resource.uri, r.offset)
	if err != nil {
		return err
	} else if len(data) == 0 && r.resource.size < 0 {
		return io.EOF
	} else if len(data) == 0 {
		return io.ErrUnexpectedEOF
	}
	r.start, r.chunk = r.offset, data
	return nil
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE FUNCTIONS

// readChunk reads the chunk of the resource's content from the offset, and
// returns it, and its URI and type, whether it's a chunk, and the size of
// all of it, which is -1 when the server doesn't say, as one which isn't a
// workspace doesn't, and returns all of it, or doesn't know it
func readChunk(ctx context.Context, c *client.Client, uri string, offset int64) ([]byte, *chunked, error) {
	result, err := c.ReadResourceParams(ctx, schema.ReadParams(uri, offset, ChunkSize))
	if err != nil {
		return nil, nil, notFound(err, uri)
	} else if len(result.Contents) == 0 || result.Contents[0] == nil {
		return nil, nil, llm.ErrNotFound.Withf("resource %q has no contents", uri)
	}
	content := result.Contents[0]
	data := content.Blob
	if data == nil {
		data = []byte(content.Text)
	}
	info := &chunked{client: c, uri: uri, mimeType: content.MIMEType, size: -1}
	if content.URI != "" {
		info.uri = content.URI
	}
	info.chunks = schema.ChunkOf(result)
	if size, ok := schema.SizeOf(result); ok {
		info.size = size
	}
	return data, info, nil
}
