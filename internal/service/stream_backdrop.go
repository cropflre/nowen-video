package service

import (
	"path/filepath"
	"strings"
)

// GetBackdropPath returns the best available backdrop for a standalone media
// item. The scanner persists BackdropPath when it discovers local artwork, but
// older databases may have been created before that field was populated, so we
// also retry the standard media-specific backdrop names beside the video.
//
// Unlike GetPosterPath this method intentionally does not fall back to a poster:
// callers can distinguish a real wide backdrop from the portrait-poster
// fallback and apply the appropriate presentation treatment.
func (s *StreamService) GetBackdropPath(mediaID string) (string, error) {
	media, err := s.mediaRepo.FindByID(mediaID)
	if err != nil {
		return "", ErrMediaNotFound
	}

	if media.BackdropPath != "" {
		if _, err := s.statMediaFile(media.BackdropPath); err == nil {
			return media.BackdropPath, nil
		}
	}

	if media.FilePath == "" {
		return "", nil
	}

	dir := vfsDir(media.FilePath)
	base := strings.TrimSuffix(filepath.Base(media.FilePath), filepath.Ext(media.FilePath))
	backdropSuffixes := []string{
		"-backdrop.jpg", "-backdrop.jpeg", "-backdrop.png", "-backdrop.webp",
		"-fanart.jpg", "-fanart.jpeg", "-fanart.png", "-fanart.webp",
		"-banner.jpg", "-banner.jpeg", "-banner.png", "-banner.webp",
	}

	for _, suffix := range backdropSuffixes {
		candidate := vfsJoinPath(dir, base+suffix)
		if _, err := s.statMediaFile(candidate); err == nil {
			return candidate, nil
		}
	}

	return "", nil
}
