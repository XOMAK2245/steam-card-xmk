package auth

import (
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"time"
)

type ProfileXML struct {
	SteamID      string `xml:"steamID"`
	AvatarIcon   string `xml:"avatarIcon"`
	AvatarMedium string `xml:"avatarMedium"`
	AvatarFull   string `xml:"avatarFull"`
}

// FetchProfile fetches public persona name and avatar URL for a given steamID64.
func FetchProfile(ctx context.Context, steamID string) (personaName, avatarURL string, err error) {
	if steamID == "" {
		return "", "", fmt.Errorf("empty steamID")
	}
	url := fmt.Sprintf("https://steamcommunity.com/profiles/%s/?xml=1", steamID)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return steamID, "", err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 XMK Suite")

	c := &http.Client{Timeout: 8 * time.Second}
	resp, err := c.Do(req)
	if err != nil {
		return steamID, "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return steamID, "", fmt.Errorf("status %d", resp.StatusCode)
	}

	b, err := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
	if err != nil {
		return steamID, "", err
	}

	var p ProfileXML
	if err := xml.Unmarshal(b, &p); err != nil {
		return steamID, "", err
	}

	personaName = p.SteamID
	if personaName == "" {
		personaName = steamID
	}
	avatarURL = p.AvatarMedium
	if avatarURL == "" {
		avatarURL = p.AvatarFull
	}
	if avatarURL == "" {
		avatarURL = p.AvatarIcon
	}
	return personaName, avatarURL, nil
}

