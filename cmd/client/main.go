package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/hajimehoshi/ebiten/v2"
	base "hsmultiplayer/content"
	"hsmultiplayer/internal/client"
	"hsmultiplayer/internal/content"
	"hsmultiplayer/internal/native"
	"hsmultiplayer/locales"
	"hsmultiplayer/pkg/lobby"
	"hsmultiplayer/pkg/localization"
	"hsmultiplayer/pkg/perf"
	"hsmultiplayer/pkg/protocol"
	"io/fs"
	"log"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"
)

func main() {
	launch := native.LaunchDefaults()
	address := flag.String("server", "127.0.0.1:7777", "server address")
	name := flag.String("name", "Player", "player name")
	windowWidth := flag.Int("width", 1280, "initial client window width")
	windowHeight := flag.Int("height", 800, "initial client window height")
	profile := flag.String("profile", "player", "local identity profile name")
	dir := flag.String("content", "", "optional external content directory")
	modRoot := flag.String("mods", launch.ModRoot, "verified mod store")
	combination := flag.String("combination", launch.Combination, "exact mod combination lock")
	sdkSource := flag.String("sdk-source", launch.Source, "bundled host/SDK source for explicit native builds")
	language := flag.String("lang", "", "language ID, overrides saved preference (zh-CN or en)")
	localeDir := flag.String("locales", "", "optional JSON language pack directory")
	fpsLimit := flag.Int("fps", 60, "render FPS limit; 0 disables the cap")
	vsync := flag.Bool("vsync", true, "synchronize rendering with display refresh")
	inputRate := flag.Int("input-rate", 30, "input sends per second: 30 or 60")
	interpolation := flag.Int("interpolation-ms", 100, "display interpolation buffer in milliseconds; 0 disables interpolation")
	projectileRenderer := flag.String("projectile-renderer", "batch", "developer comparison: batch (GPU shader) or sprites (cached images)")
	performance := flag.Bool("performance", false, "show measured FPS and RTT (F6); detailed timings in Settings")
	metrics := flag.String("metrics", "", "write bounded performance JSON every second and on exit")
	autoplay := flag.Bool("smoke-autoplay", false, "connect, start and exercise combat input during smoke validation only")
	smokeNode := flag.String("smoke-node", "", "combat node for smoke validation, or movement item ID/definition with -smoke-map (optional)")
	hostTPS := flag.Int("host-tps", 30, "local host simulation ticks per second: 30 or 60")
	hostSnapshots := flag.Int("host-snapshot-rate", 20, "local host snapshots per second (1..60)")
	smoke := flag.Int("smoke-frames", 0, "exit after N updates (render smoke check)")
	smokeSeconds := flag.Duration("smoke-duration", 0, "optional wall-clock bound for explicit smoke runs, e.g. 20s")
	shot := flag.String("screenshot", "", "save a render smoke PNG")
	state := flag.String("smoke-state", "", "snapshot file for render validation")
	mapView := flag.Bool("smoke-map", false, "render snapshot map")
	rotation := flag.Float64("smoke-rotation", 0, "camera rotation in degrees relative to the default point-up view")
	zoom := flag.Float64("smoke-zoom", 1, "camera zoom during render validation")
	details := flag.Bool("smoke-details", false, "render snapshot damage explanation")
	detailOffset := flag.Int("smoke-detail-offset", 0, "first weapon-window detail row during render smoke validation")
	contentOffset := flag.Int("smoke-content-offset", 0, "window body scroll offset in pixels during render smoke validation")
	smokeWindow := flag.String("smoke-window", "", "open a native window or layout HUD editor during smoke validation")
	smokeHover := flag.String("smoke-hover", "", "focus a native Hover during smoke validation")
	smokePage := flag.String("smoke-page", "", "main or lobby during native render validation")
	smokeAction := flag.String("smoke-hall-action", "", "walk to solo, host or join device during native validation")
	cpuProfile := flag.String("cpuprofile", "", "write developer CPU profile on exit")
	flag.Parse()
	stopProfile, profileErr := perf.StartCPUProfile(*cpuProfile)
	if profileErr != nil {
		log.Fatal(profileErr)
	}
	defer stopProfile()
	if *projectileRenderer != "batch" && *projectileRenderer != "sprites" {
		log.Fatal("projectile renderer must be batch or sprites")
	}
	if *smokeSeconds < 0 || *smokeSeconds > 10*time.Minute {
		log.Fatal("smoke duration must be 0..10m")
	}
	if *smokePage != "" && *smokePage != "main" && *smokePage != "lobby" || *smokeAction != "" && *smokeAction != "solo" && *smokeAction != "host" && *smokeAction != "join" {
		log.Fatal("invalid smoke front page/action")
	}
	if *windowWidth < 640 || *windowWidth > 4096 || *windowHeight < 480 || *windowHeight > 2160 {
		log.Fatal("invalid window dimensions")
	}
	if err := client.ValidateTiming(*fpsLimit, *inputRate, time.Duration(*interpolation)*time.Millisecond); err != nil {
		log.Fatal(err)
	}
	if (*hostTPS != 30 && *hostTPS != 60) || *hostSnapshots < 1 || *hostSnapshots > 60 {
		log.Fatal("invalid local host timing")
	}
	fpsOverride, vsyncOverride := false, false
	nameOverride := false
	flag.Visit(func(f *flag.Flag) {
		if f.Name == "name" {
			nameOverride = true
		}
		if f.Name == "fps" {
			fpsOverride = true
		}
		if f.Name == "vsync" {
			vsyncOverride = true
		}
	})
	if strings.ContainsAny(*profile, "/\\:") || *profile == "" || *profile == ".." {
		log.Fatal("invalid profile name")
	}
	var c *content.Catalog
	var err error
	if *dir == "" {
		c, err = content.LoadWithOptions(base.Files, content.LoadOptions{Client: true})
	} else {
		c, err = content.LoadWithOptions(os.DirFS(*dir), content.LoadOptions{Client: true})
	}
	if err != nil {
		log.Fatal(err)
	}
	var baseFiles fs.FS = base.Files
	if *dir != "" {
		baseFiles = os.DirFS(*dir)
	}
	reload := func(path string, previous *content.Catalog) (*content.Catalog, error) {
		if path == "" {
			return content.LoadWithOptions(baseFiles, content.LoadOptions{Client: true, Previous: previous})
		}
		lock, err := content.LoadLock(path)
		if err != nil {
			return nil, err
		}
		return content.ComposeCached(baseFiles, *modRoot, lock, true, previous)
	}
	if *combination != "" {
		c, err = reload(*combination, nil)
		if err != nil {
			log.Fatal(err)
		}
	}
	localeSources := []fs.FS{locales.Files}
	if *localeDir == "" {
		if exe, err := os.Executable(); err == nil {
			candidate := filepath.Join(filepath.Dir(exe), "locales")
			if info, err := os.Stat(candidate); err == nil && info.IsDir() {
				*localeDir = candidate
			}
		}
	}
	if *localeDir != "" {
		localeSources = append(localeSources, os.DirFS(*localeDir))
	}
	translations, err := localization.Load(localeSources...)
	if err != nil {
		log.Fatal(err)
	}
	if *language != "" && !translations.HasLanguage(*language) {
		log.Fatalf("unknown language %q; available: %v", *language, translations.Languages())
	}
	var host *exec.Cmd
	var hostMu sync.Mutex
	stopHost := func() {
		hostMu.Lock()
		defer hostMu.Unlock()
		if host != nil && host.Process != nil {
			_ = host.Process.Kill()
		}
		host = nil
	}
	defer stopHost()
	startHost := func(options lobby.Config, solo bool) error {
		if err := options.Validate(); err != nil {
			return err
		}
		hostMu.Lock()
		defer hostMu.Unlock()
		if host != nil {
			return protocol.Reject("error.host_exists")
		}
		exe, err := os.Executable()
		if err != nil {
			return err
		}
		path := filepath.Join(filepath.Dir(exe), "server.exe")
		if _, err = os.Stat(path); err != nil {
			path = filepath.Join("bin", "server.exe")
		}
		cmd := exec.Command(path)
		listen := fmt.Sprintf("0.0.0.0:%d", options.Port)
		if solo {
			listen = fmt.Sprintf("127.0.0.1:%d", options.Port)
		}
		checkpoint := filepath.Join("data", "hosts", *profile, "checkpoint.json")
		cmd.Args = append(cmd.Args, "-listen", listen, "-players", fmt.Sprint(options.Players), "-vote-ratio", fmt.Sprint(float64(options.VotePercent)/100), "-tps", fmt.Sprint(options.TickRate), "-snapshot-rate", fmt.Sprint(options.SnapshotRate), "-checkpoint", checkpoint)
		data, err := json.Marshal(options)
		if err != nil {
			return err
		}
		cmd.Env = append(os.Environ(), "HSM_ROOM_CONFIG="+string(data), "HSM_ADMIN_TOKEN="+options.AdminToken)
		if options.Restore {
			restorePath := options.Checkpoint
			if restorePath == "" {
				restorePath = checkpoint
			}
			cmd.Args = append(cmd.Args, "-restore", restorePath)
		}
		if options.Combination != "" {
			cmd.Args = append(cmd.Args, "-combination", options.Combination, "-mods", *modRoot)
		}
		if *dir != "" {
			cmd.Args = append(cmd.Args, "-content", *dir)
		}
		cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
		stderr, err := cmd.StderrPipe()
		if err != nil {
			return err
		}
		if err = cmd.Start(); err != nil {
			return err
		}
		host = cmd
		ready := make(chan error, 1)
		go func() {
			scanner := bufio.NewScanner(stderr)
			announced := false
			for scanner.Scan() {
				line := scanner.Text()
				log.Print(line)
				if !announced && strings.Contains(line, "HSMultiplayer server ") {
					announced = true
					ready <- nil
				}
			}
			err := cmd.Wait()
			if !announced {
				if err == nil {
					err = fmt.Errorf("server exited before listening")
				}
				ready <- err
			}
		}()
		select {
		case err := <-ready:
			if err != nil {
				host = nil
			}
			return err
		case <-time.After(5 * time.Second):
			_ = cmd.Process.Kill()
			host = nil
			return fmt.Errorf("server startup timed out")
		}
	}
	options := lobby.DefaultConfig()
	options.Combination = *combination
	build := func(path string) error {
		source := *sdkSource
		if source == "." {
			if _, err := os.Stat("go.mod"); os.IsNotExist(err) {
				exe, err := os.Executable()
				if err != nil {
					return err
				}
				source = filepath.Join(filepath.Dir(exe), "sdk-source")
			}
		}
		return content.BuildNative(source, *modRoot, path, filepath.Join("bin", "modded"))
	}
	options.TickRate = *hostTPS
	options.SnapshotRate = *hostSnapshots
	g := client.New(c, client.Config{ProjectileRenderer: *projectileRenderer, ModRoot: *modRoot, Combination: *combination, ReloadContent: reload, BuildContent: build, FrameLimit: *fpsLimit, FPSOverride: fpsOverride, VSync: *vsync, VSyncOverride: vsyncOverride, InputRate: *inputRate, InterpolationDelay: time.Duration(*interpolation) * time.Millisecond, ShowPerformance: *performance || *metrics != "", Metrics: *metrics, SmokeAutoplay: *autoplay, SmokeNode: *smokeNode, Language: *language, Translations: translations, Address: *address, Name: *name, NameOverride: nameOverride, Profile: filepath.Join("data", "profiles", *profile+".json"), Host: startHost, StopHost: stopHost, RoomConfig: options, SmokePage: *smokePage, SmokeAction: *smokeAction, SmokeFrames: *smoke, SmokeDuration: *smokeSeconds, Screenshot: *shot, SmokeState: *state, SmokeMap: *mapView, SmokeRotation: *rotation * math.Pi / 180, SmokeZoom: *zoom, SmokeDetails: *details, SmokeDetailOffset: *detailOffset, SmokeContentOffset: *contentOffset, SmokeWindow: *smokeWindow, SmokeHover: *smokeHover})
	defer g.Close()
	ebiten.SetWindowSize(*windowWidth, *windowHeight)
	ebiten.SetWindowResizingMode(ebiten.WindowResizingModeEnabled)
	ebiten.SetRunnableOnUnfocused(true)
	ebiten.SetTPS(60)
	if err = ebiten.RunGame(g); err != nil {
		g.Close()
		stopProfile()
		log.Fatal(err)
	}
}
