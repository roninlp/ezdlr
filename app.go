package main

import "context"

type App struct {
	service *DownloadService
}

func NewApp(service *DownloadService) *App {
	return &App{service: service}
}

func (a *App) startup(context.Context) {}

func (a *App) shutdown(context.Context) {
	_ = a.service.Shutdown()
}

func (a *App) AddURL(url string) (DownloadItem, error) {
	return a.service.AddURL(url)
}

func (a *App) Snapshot() ServiceSnapshot {
	return a.service.Snapshot()
}

func (a *App) Configuration() Configuration {
	return a.service.Configuration()
}
