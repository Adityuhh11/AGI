// Command agi is the launcher: a global hotkey opens a search bar over the memory engine's index.
package main

import (
	"io/fs"
	"log"

	"github.com/wailsapp/wails/v3/pkg/application"

	"agi"
	"agi/internal/config"
	"agi/internal/status"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	path, err := config.Path()
	if err != nil {
		return err
	}
	cfg, err := config.Load(path)
	if err != nil {
		return err
	}
	assets, err := fs.Sub(agi.Frontend, "frontend/dist")
	if err != nil {
		return err
	}

	app := application.New(application.Options{
		Name: "AGI",
		Services: []application.Service{
			application.NewService(status.New(status.Info{Hotkey: cfg.Hotkey, Roots: cfg.Roots})),
		},
		Assets: application.AssetOptions{Handler: application.AssetFileServerFS(assets)},
	})
	app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:            "AGI",
		Width:            720,
		Height:           420,
		BackgroundColour: application.NewRGB(0, 0, 0),
		URL:              "/",
	})
	return app.Run()
}
