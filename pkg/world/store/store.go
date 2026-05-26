// Package store exposes world persistence contracts and file-backed
// storage for downstream repositories.
package store

import internal "github.com/sizolity/nobody/internal/world/store"

type Store = internal.Store
type FileStore = internal.FileStore

func NewFileStore(workspace string) *FileStore {
	return internal.NewFileStore(workspace)
}
