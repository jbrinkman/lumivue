package main

import (
	"embed"
	"io/fs"
	"log"

	"github.com/wailsapp/wails/v3/pkg/application"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	logCloser := initLogger()
	defer logCloser.Close()

	appService := NewAppService()

	subFS, err := fs.Sub(assets, "frontend/dist")
	if err != nil {
		log.Fatal(err)
	}

	app := application.New(application.Options{
		Name:        "Lumivue",
		Description: "Camera viewer application",
		Assets: application.AssetOptions{
			Handler: application.BundledAssetFileServer(subFS),
		},
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: true,
		},
		Services: []application.Service{
			application.NewService(appService),
		},
	})

	app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:            "Lumivue",
		Name:             "main",
		Width:            1200,
		Height:           800,
		BackgroundColour: application.NewRGBA(15, 15, 20, 255),
		Mac: application.MacWindow{
			Backdrop:                application.MacBackdropTranslucent,
			TitleBar:                application.MacTitleBarHiddenInset,
			InvisibleTitleBarHeight: 44,
		},
	})

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}
