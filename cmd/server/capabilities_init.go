package main

import (
	"github.com/nowen-video/nowen-video/internal/config"
	"github.com/nowen-video/nowen-video/internal/middleware"
	"github.com/nowen-video/nowen-video/internal/serverprofile"
)

func init() {
	middleware.SetPublicCapabilitiesProvider(func() (serverprofile.Manifest, error) {
		cfg, err := config.Load()
		if err != nil {
			return serverprofile.Manifest{}, err
		}
		return serverprofile.Full(cfg), nil
	})
}
