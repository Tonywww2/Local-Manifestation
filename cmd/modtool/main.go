package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"hsmultiplayer/internal/content"
	"hsmultiplayer/pkg/mods"
	"os"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
func run(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: modtool pack -dir PATH -out FILE | verify -file FILE -sha256 HASH | validate -dir PATH | validate -file FILE -sha256 HASH")
	}
	f := flag.NewFlagSet(args[0], flag.ContinueOnError)
	switch args[0] {
	case "build":
		source := f.String("source", ".", "host/SDK source directory")
		root := f.String("root", "data/mods", "installed packages")
		lock := f.String("lock", "data/combinations/default.json", "exact combination lock")
		out := f.String("out", "bin/modded", "linked programs")
		if err := f.Parse(args[1:]); err != nil {
			return err
		}
		return content.BuildNative(*source, *root, *lock, *out)
	case "install":
		path := f.String("file", "", "package")
		hash := f.String("sha256", "", "required SHA-256")
		root := f.String("root", "data/mods", "installation store")
		if err := f.Parse(args[1:]); err != nil {
			return err
		}
		record, err := content.InstallPackage(*root, *path, *hash)
		if err != nil {
			return err
		}
		return json.NewEncoder(os.Stdout).Encode(record)
	case "list":
		root := f.String("root", "data/mods", "installation store")
		if err := f.Parse(args[1:]); err != nil {
			return err
		}
		values, err := content.Installed(*root)
		if err != nil {
			return err
		}
		return json.NewEncoder(os.Stdout).Encode(values)
	case "resolve":
		root := f.String("root", "data/mods", "installation store")
		out := f.String("out", "data/combinations/default.json", "combination lock")
		if err := f.Parse(args[1:]); err != nil {
			return err
		}
		installed, err := content.Installed(*root)
		if err != nil {
			return err
		}
		var available []mods.Package
		for _, p := range installed {
			available = append(available, p.Package)
		}
		previous, _ := content.LoadLock(*out)
		lock, err := mods.Resolve(available, f.Args(), previous)
		if err != nil {
			return err
		}
		if err = content.SaveLock(*out, lock); err != nil {
			return err
		}
		return json.NewEncoder(os.Stdout).Encode(lock)
	case "validate":
		dir := f.String("dir", "", "source content directory")
		path := f.String("file", "", "verified package")
		hash := f.String("sha256", "", "required package SHA-256")
		if err := f.Parse(args[1:]); err != nil {
			return err
		}
		if (*dir == "") == (*path == "") {
			return fmt.Errorf("choose exactly one of -dir or -file")
		}
		var catalog *content.Catalog
		var err error
		if *dir != "" {
			catalog, err = content.Load(os.DirFS(*dir))
		} else {
			archive, _, verifyErr := content.ReadPackage(*path, *hash)
			if verifyErr != nil {
				return verifyErr
			}
			catalog, err = content.Load(archive)
		}
		if err != nil {
			return err
		}
		return json.NewEncoder(os.Stdout).Encode(struct {
			Digest  string          `json:"digest"`
			Timings content.Timings `json:"timings"`
		}{catalog.Digest, catalog.Timings()})
	case "pack":
		dir := f.String("dir", "", "source directory")
		out := f.String("out", "", "output archive")
		if err := f.Parse(args[1:]); err != nil {
			return err
		}
		if *dir == "" || *out == "" {
			return fmt.Errorf("-dir and -out required")
		}
		b, info, err := content.Pack(os.DirFS(*dir))
		if err != nil {
			return err
		}
		if err = os.WriteFile(*out, b, 0644); err != nil {
			return err
		}
		if err = os.WriteFile(*out+".sha256", []byte(info.SHA256+"\n"), 0644); err != nil {
			return err
		}
		return json.NewEncoder(os.Stdout).Encode(info)
	case "verify":
		path := f.String("file", "", "archive")
		hash := f.String("sha256", "", "expected SHA-256")
		if err := f.Parse(args[1:]); err != nil {
			return err
		}
		_, info, err := content.ReadPackage(*path, *hash)
		if err != nil {
			return err
		}
		return json.NewEncoder(os.Stdout).Encode(info)
	default:
		return fmt.Errorf("unknown command %s", args[0])
	}
}
