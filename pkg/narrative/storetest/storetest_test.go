package storetest_test

import (
	"testing"

	"github.com/sizolity/nobody/pkg/narrative/store"
	"github.com/sizolity/nobody/pkg/narrative/storetest"
)

func TestFileStoreSatisfiesPublicStoreContract(t *testing.T) {
	storetest.RunStoreContract(t, func(t testing.TB) store.Store {
		t.Helper()
		return store.NewFileStore(t.TempDir())
	})
}
