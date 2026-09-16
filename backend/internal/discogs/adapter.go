package discogs

import (
	"context"

	"github.com/julianhuber/hitsync/backend/internal/years"
)

// YearsAdapter adapts *Client to the years.DiscogsResolver interface,
// converting between the two packages' Result types (they are kept separate
// to avoid an import cycle, since this package already depends on years for
// normalisation).
type YearsAdapter struct {
	Client *Client
}

// Resolve implements years.DiscogsResolver.
func (a YearsAdapter) Resolve(ctx context.Context, title, artist string) (*years.DiscogsResult, error) {
	res, err := a.Client.Resolve(ctx, title, artist)
	if err != nil || res == nil {
		return nil, err
	}
	return &years.DiscogsResult{Year: res.Year, Source: res.Source}, nil
}
