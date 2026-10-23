package main

import (
	"context"
	"encoding/json"
	"flag"
	base "hsmultiplayer/content"
	"hsmultiplayer/internal/content"
	"hsmultiplayer/internal/native"
	"hsmultiplayer/internal/persistence"
	"hsmultiplayer/internal/server"
	"hsmultiplayer/internal/session"
	"hsmultiplayer/pkg/lobby"
	"hsmultiplayer/pkg/mods"
	"hsmultiplayer/pkg/perf"
	"io/fs"
	"log"
	"os"
	"os/signal"
	"time"
)

func main() {
	launch := native.LaunchDefaults()
	addr := flag.String("listen", "0.0.0.0:7777", "TCP listen address")
	max := flag.Int("players", 8, "maximum players")
	logicRate := flag.Int("tps", 30, "fixed simulation ticks per second: 30 or 60")
	snapshotRate := flag.Int("snapshot-rate", 20, "network snapshots per second (1..60)")
	ratio := flag.Float64("vote-ratio", 0.7, "team vote threshold (0,1]")
	seed := flag.Int64("seed", time.Now().UnixNano(), "adventure seed")
	contentDir := flag.String("content", "", "external content directory (optional)")
	save := flag.String("checkpoint", "data/checkpoint.json", "checkpoint path")
	restore := flag.String("restore", "", "restore checkpoint path")
	modRoot := flag.String("mods", launch.ModRoot, "verified mod package store")
	combination := flag.String("combination", launch.Combination, "exact package lock")
	cpuProfile := flag.String("cpuprofile", "", "write developer CPU profile on exit")
	metrics := flag.String("metrics", "", "write bounded server/room performance JSON every second")
	flag.Parse()
	stopProfile, profileErr := perf.StartCPUProfile(*cpuProfile)
	if profileErr != nil {
		log.Fatal(profileErr)
	}
	defer stopProfile()
	var options lobby.Config
	if data := os.Getenv("HSM_ROOM_CONFIG"); data != "" {
		if err := json.Unmarshal([]byte(data), &options); err != nil || options.Validate() != nil {
			log.Fatal("invalid HSM_ROOM_CONFIG")
		}
	}
	var c *content.Catalog
	var err error
	if *contentDir == "" {
		c, err = content.Load(base.Files)
	} else {
		c, err = content.Load(os.DirFS(*contentDir))
	}
	if err != nil {
		log.Fatal(err)
	}
	lock := mods.Lock{Version: 1}
	if *combination != "" {
		lock, err = content.LoadLock(*combination)
		if err != nil {
			log.Fatal(err)
		}
		var baseFiles fs.FS = base.Files
		if *contentDir != "" {
			baseFiles = os.DirFS(*contentDir)
		}
		c, err = content.Compose(baseFiles, *modRoot, lock, false)
		if err != nil {
			log.Fatal(err)
		}
	}
	cfg := session.Config{TickRate: *logicRate, SnapshotRate: *snapshotRate, MaxPlayers: *max, Seed: *seed, Checkpoint: *save, VoteRatio: *ratio}
	cfg.AdminToken = os.Getenv("HSM_ADMIN_TOKEN")
	cfg.SpecialMode = options.SpecialMode
	cfg.BackpackCapacity = options.BackpackCapacity
	cfg.AccessorySlots = options.AccessorySlots
	cfg.MetaRules = options.MetaRules
	if *restore != "" {
		saved, err := persistence.Load(*restore, c.Digest)
		if err != nil {
			log.Fatal(err)
		}
		cfg.Restore = &saved
	}
	co, err := session.New(c, cfg)
	if err != nil {
		log.Fatal(err)
	}
	defer co.Close()
	listener, err := server.Listen(*addr)
	if err != nil {
		log.Fatal(err)
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	stopMonitor := monitorServer(ctx, co, *metrics)
	defer stopMonitor()
	log.Printf("HSMultiplayer server %s; content %s; seed %d; logic %d TPS; snapshots %d Hz", listener.Addr(), c.Digest, *seed, *logicRate, *snapshotRate)
	if err = server.Serve(ctx, listener, co, server.Distribution{Root: *modRoot, Lock: lock}); err != nil {
		log.Fatal(err)
	}
}
