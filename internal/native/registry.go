// Package native contains only immutable, statically linked registration factories.
package native

import (
	"fmt"
	"hsmultiplayer/sdk"
)

type Factory func(sdk.Registrar) error
type Entry struct {
	ID, Version, CodeSHA256 string
	Shared, Server, Client  Factory
}

var entries = map[string]Entry{}

// Add is called only from linked module init functions, before any world exists.
func Add(e Entry) {
	key := e.ID + "@" + e.Version
	if _, ok := entries[key]; ok {
		panic("duplicate native module")
	}
	entries[key] = e
}
func Lookup(id, version, hash string, client bool) (Factory, error) {
	e, ok := entries[id+"@"+version]
	if !ok || e.CodeSHA256 != hash {
		return nil, fmt.Errorf("Go module %s@%s requires rebuilding this host", id, version)
	}
	return func(r sdk.Registrar) error {
		if e.Shared != nil {
			if err := e.Shared(r); err != nil {
				return err
			}
		}
		side := e.Server
		if client {
			side = e.Client
		}
		if side != nil {
			return side(r)
		}
		return nil
	}, nil
}
