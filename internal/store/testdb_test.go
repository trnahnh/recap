package store

import (
	"context"
	"testing"
	"time"

	"github.com/trnahnh/recap/internal/testdb"
)

func newTestStore(t *testing.T) (*Store, context.Context) {
	t.Helper()
	_, pool := testdb.Setup(t)
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	t.Cleanup(cancel)
	return New(pool), ctx
}
