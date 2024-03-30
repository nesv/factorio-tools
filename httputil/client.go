// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package httputil

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"
)

// UserAgent is the user agent used in all requests to any Factorio API.
const UserAgent = "factorio-tools/0.1"

var (
	clientOnce sync.Once
	client     *http.Client
)

// Client returns a [net/http.Client] that will set the "user-agent" header to
// [UserAgent] for all requests.
// Similar to [net/http.DefaultClient], the returned client will stop after 10
// redirects.
// Requests will timeout after 1m.
// Multiple calls to Client will return the same client.
func Client() *http.Client {
	clientOnce.Do(func() {
		client = &http.Client{
			Transport: http.DefaultTransport,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				if len(via) > 10 {
					return errors.New("stopped after 10 redirects")
				}
				req.Header.Set("user-agent", UserAgent)
				return nil
			},
			Timeout: time.Minute,
		}
	})
	return client
}

func Get(ctx context.Context, urlStr string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, urlStr, nil)
	if err != nil {
		return nil, fmt.Errorf("new request: %w", err)
	}
	req.Header.Set("user-agent", UserAgent)
	return Client().Do(req)
}
