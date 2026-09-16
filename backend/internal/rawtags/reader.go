package rawtags

import (
	"context"
	"fmt"
	"io"
	"net/http"
)

// chunkSize is the unit of HTTP Range fetches. dhowden/tag issues many small
// Read/Seek calls while parsing a tag block; batching them into chunks keeps
// the number of round trips to Navidrome low.
const chunkSize = 64 * 1024

// maxChunks bounds total memory per reader (a file with a large embedded
// cover art frame can otherwise pull in several megabytes).
const maxChunks = 512

// rangeReadSeeker is an io.ReadSeeker over an HTTP resource, fetching only
// the byte ranges the caller actually touches (via cached chunkSize blocks)
// rather than downloading the whole file.
type rangeReadSeeker struct {
	ctx    context.Context
	client *http.Client
	url    string
	size   int64
	pos    int64
	chunks map[int64][]byte
}

func newRangeReadSeeker(ctx context.Context, client *http.Client, url string) (*rangeReadSeeker, error) {
	size, err := fetchSize(ctx, client, url)
	if err != nil {
		return nil, err
	}
	return &rangeReadSeeker{ctx: ctx, client: client, url: url, size: size, chunks: make(map[int64][]byte)}, nil
}

func fetchSize(ctx context.Context, client *http.Client, url string) (int64, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, url, nil)
	if err != nil {
		return 0, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	resp.Body.Close()
	if resp.StatusCode == http.StatusOK && resp.ContentLength > 0 {
		return resp.ContentLength, nil
	}

	// Some servers do not answer HEAD requests usefully; fall back to a
	// single-byte ranged GET and read the total size off Content-Range.
	req, err = http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return 0, err
	}
	req.Header.Set("Range", "bytes=0-0")
	resp, err = client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)

	if resp.StatusCode != http.StatusPartialContent {
		return 0, fmt.Errorf("rawtags: server does not support range requests (status %d)", resp.StatusCode)
	}
	var total int64
	if _, err := fmt.Sscanf(resp.Header.Get("Content-Range"), "bytes 0-0/%d", &total); err != nil {
		return 0, fmt.Errorf("rawtags: parsing Content-Range: %w", err)
	}
	return total, nil
}

func (r *rangeReadSeeker) Seek(offset int64, whence int) (int64, error) {
	var newPos int64
	switch whence {
	case io.SeekStart:
		newPos = offset
	case io.SeekCurrent:
		newPos = r.pos + offset
	case io.SeekEnd:
		newPos = r.size + offset
	default:
		return 0, fmt.Errorf("rawtags: invalid whence %d", whence)
	}
	if newPos < 0 {
		return 0, fmt.Errorf("rawtags: negative seek position")
	}
	r.pos = newPos
	return newPos, nil
}

func (r *rangeReadSeeker) Read(p []byte) (int, error) {
	if r.pos >= r.size {
		return 0, io.EOF
	}
	n := 0
	for n < len(p) && r.pos < r.size {
		idx := r.pos / chunkSize
		chunk, err := r.chunk(idx)
		if err != nil {
			return n, err
		}
		offsetInChunk := r.pos % chunkSize
		if offsetInChunk >= int64(len(chunk)) {
			// Short chunk at EOF and we've already consumed it.
			break
		}
		copied := copy(p[n:], chunk[offsetInChunk:])
		n += copied
		r.pos += int64(copied)
	}
	if n == 0 {
		return 0, io.EOF
	}
	return n, nil
}

func (r *rangeReadSeeker) chunk(idx int64) ([]byte, error) {
	if c, ok := r.chunks[idx]; ok {
		return c, nil
	}
	if len(r.chunks) >= maxChunks {
		return nil, fmt.Errorf("rawtags: exceeded max chunk budget reading tags")
	}

	start := idx * chunkSize
	end := start + chunkSize - 1
	if end >= r.size {
		end = r.size - 1
	}

	req, err := http.NewRequestWithContext(r.ctx, http.MethodGet, r.url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", start, end))
	resp, err := r.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusPartialContent && resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("rawtags: range request returned status %d", resp.StatusCode)
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	r.chunks[idx] = data
	return data, nil
}
