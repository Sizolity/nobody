// Package store exposes narrative persistence contracts and file-backed
// storage for downstream product repositories.
package store

import internal "github.com/sizolity/nobody/internal/narrative/store"

type Store = internal.Store
type FileStore = internal.FileStore

func NewFileStore(workspace string) *FileStore {
	return internal.NewFileStore(workspace)
}
