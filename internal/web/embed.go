package web

import (
	"embed"
	"io/fs"
)

//go:embed dist/front/* dist/admin/* templates/*
var assets embed.FS

func Front() fs.FS {
	sub, _ := fs.Sub(assets, "dist/front")
	return sub
}

func Admin() fs.FS {
	sub, _ := fs.Sub(assets, "dist/admin")
	return sub
}

func Templates() fs.FS {
	sub, _ := fs.Sub(assets, "templates")
	return sub
}

