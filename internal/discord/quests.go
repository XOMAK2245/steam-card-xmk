package discord

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand/v2"
	"net/http"
	"strconv"
	"time"
)

const apiBase = "https://discord.com/api/v9/"

// ErrUnauthorized is returned when the Discord token is invalid or expired.
var ErrUnauthorized = errors.New("discord: invalid token (401)")

// ---------- Quest data types ----------

// QuestTaskConfig describes what the user must do for a quest.
type QuestTaskConfig struct {
	ApplicationID string `json:"application_id"`
	Duration      int    `json:"stream_duration_requirement_minutes"`
	TaskType      string `json:"type"` // PLAY_ON_DESKTOP, STREAM_ON_DESKTOP, WATCH_VIDEO
}

// QuestReward contains reward description.
type QuestReward struct {
	Messages struct {
		Completed string `json:"completed"`
	} `json:"messages"`
}

// QuestUserStatus tracks user progress on a quest.
type QuestUserStatus struct {
	EnrolledAt            string `json:"enrolled_at"`
	CompletedAt           string `json:"completed_at"`
	ClaimedAt             string `json:"claimed_at"`
	LastStreamHeartbeatAt string `json:"last_stream_heartbeat_at"`
	Progress              struct {
		StreamProgressSeconds int  `json:"stream_progress_seconds"`
		Done                  bool `json:"done"`
	} `json:"progress"`
}

// QuestConfig holds quest metadata.
type QuestConfig struct {
	ExpiresAt    string          `json:"expires_at"`
	TaskConfig   QuestTaskConfig `json:"quest_task_config"`
	RewardConfig QuestReward     `json:"reward_config"`
	Messages     struct {
		QuestName string `json:"quest_name"`
	} `json:"messages"`
}

// Quest represents a single Discord quest.
type Quest struct {
	ID         string           `json:"id"`
	Config     QuestConfig      `json:"config"`
	UserStatus *QuestUserStatus `json:"user_status"`
}

// Name returns the human-readable quest name.
func (q Quest) Name() string { return q.Config.Messages.QuestName }

// IsStream returns true if the quest type involves streaming or video.
func (q Quest) IsStream() bool {
	t := q.Config.TaskConfig.TaskType
	return t == "STREAM_ON_DESKTOP" || t == "WATCH_VIDEO"
}

// ---------- Client ----------

// Client interacts with the Discord Quest API.
type Client struct {
	token string
	hc    *http.Client
	logf  func(string, ...any)
}

// NewClient creates a Discord API client.
func NewClient(token string, logf func(string, ...any)) *Client {
	if logf == nil {
		logf = func(string, ...any) {}
	}
	return &Client{
		token: token,
		hc:    &http.Client{Timeout: 30 * time.Second},
		logf:  logf,
	}
}

// doReq performs an HTTP request with Discord headers. It retries on 429.
func (c *Client) doReq(ctx context.Context, method, path string, body any) (*http.Response, error) {
	var bodyReader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		bodyReader = bytes.NewReader(b)
	}

	for attempt := 0; attempt < 4; attempt++ {
		req, err := http.NewRequestWithContext(ctx, method, apiBase+path, bodyReader)
		if err != nil {
			return nil, err
		}
		for k, vs := range DefaultHeaders(c.token) {
			for _, v := range vs {
				req.Header.Set(k, v)
			}
		}
		if body == nil && method != "GET" {
			req.Header.Set("Content-Type", "application/json")
		}

		resp, err := c.hc.Do(req)
		if err != nil {
			return nil, err
		}

		if resp.StatusCode == http.StatusUnauthorized {
			resp.Body.Close()
			return nil, ErrUnauthorized
		}

		if resp.StatusCode == http.StatusTooManyRequests {
			resp.Body.Close()
			wait := 5 * time.Second
			if ra := resp.Header.Get("Retry-After"); ra != "" {
				if secs, err := strconv.ParseFloat(ra, 64); err == nil {
					wait = time.Duration(secs*1000) * time.Millisecond
				}
			}
			c.logf("[Discord] Rate limited, waiting %v", wait)
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(wait):
			}
			// Reset body reader for retry
			if body != nil {
				b, _ := json.Marshal(body)
				bodyReader = bytes.NewReader(b)
			}
			continue
		}

		return resp, nil
	}

	return nil, fmt.Errorf("discord: too many retries")
}

type questsResponse struct {
	Quests []Quest `json:"quests"`
}

// ParseQuests decodes Discord quests from JSON bytes, supporting both {"quests": [...]} and raw array [...].
func ParseQuests(data []byte) ([]Quest, error) {
	// 1. Try Discord's standard object format: {"quests": [...]}
	var res questsResponse
	if err := json.Unmarshal(data, &res); err == nil && res.Quests != nil {
		return res.Quests, nil
	}

	// 2. Try raw array format: [...]
	var list []Quest
	if err := json.Unmarshal(data, &list); err == nil {
		if list == nil {
			list = []Quest{}
		}
		return list, nil
	}

	return nil, fmt.Errorf("unexpected quests JSON structure")
}

// FetchQuests retrieves available quests for the authenticated user.
func (c *Client) FetchQuests(ctx context.Context) ([]Quest, error) {
	resp, err := c.doReq(ctx, "GET", "quests/@me", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return nil, fmt.Errorf("discord: fetch quests: %d %s", resp.StatusCode, string(b))
	}

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("discord: read quests: %w", err)
	}

	quests, err := ParseQuests(b)
	if err != nil {
		return nil, fmt.Errorf("discord: parse quests: %w", err)
	}
	if quests == nil {
		quests = []Quest{}
	}
	return quests, nil
}

// Enroll accepts a quest (with realistic delay).
func (c *Client) Enroll(ctx context.Context, questID string) error {
	jitter := time.Duration(1000+rand.IntN(2000)) * time.Millisecond
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(jitter):
	}

	resp, err := c.doReq(ctx, "POST", "quests/"+questID+"/enroll", struct{}{})
	if err != nil {
		return err
	}
	resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("discord: enroll: HTTP %d", resp.StatusCode)
	}
	return nil
}

// Heartbeat sends a single progress heartbeat for a quest.
func (c *Client) Heartbeat(ctx context.Context, questID string, isStream bool) error {
	body := map[string]bool{"stream_heartbeat": isStream}
	resp, err := c.doReq(ctx, "POST", "quests/"+questID+"/heartbeat", body)
	if err != nil {
		return err
	}
	resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("discord: heartbeat: HTTP %d", resp.StatusCode)
	}
	return nil
}

// Claim claims the reward for a completed quest (with realistic delay).
func (c *Client) Claim(ctx context.Context, questID string) error {
	jitter := time.Duration(1000+rand.IntN(2000)) * time.Millisecond
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(jitter):
	}

	resp, err := c.doReq(ctx, "POST", "quests/"+questID+"/claim", struct{}{})
	if err != nil {
		return err
	}
	resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("discord: claim: HTTP %d", resp.StatusCode)
	}
	return nil
}

// HeartbeatInterval returns 30 seconds ± random jitter for realistic timing.
func HeartbeatInterval() time.Duration {
	return 30*time.Second + time.Duration(rand.IntN(7)-3)*time.Second // 27-33s
}

// RunQuest runs a quest to completion: enroll → heartbeat loop → claim.
// onProgress is called after each heartbeat with (elapsed, total) durations.
func (c *Client) RunQuest(ctx context.Context, quest Quest, onProgress func(elapsed, total time.Duration)) error {
	totalDur := time.Duration(quest.Config.TaskConfig.Duration) * time.Minute
	if totalDur == 0 {
		totalDur = 15 * time.Minute // sensible default
	}
	isStream := quest.IsStream()

	// Enroll if not already enrolled
	if quest.UserStatus == nil || quest.UserStatus.EnrolledAt == "" {
		c.logf("[Discord] Enrolling in quest: %s", quest.Name())
		if err := c.Enroll(ctx, quest.ID); err != nil {
			return fmt.Errorf("enroll: %w", err)
		}
	}

	// Calculate already-elapsed progress
	var alreadyElapsed time.Duration
	if quest.UserStatus != nil {
		alreadyElapsed = time.Duration(quest.UserStatus.Progress.StreamProgressSeconds) * time.Second
	}
	if alreadyElapsed >= totalDur {
		c.logf("[Discord] Quest already completed, claiming reward")
		return c.Claim(ctx, quest.ID)
	}

	c.logf("[Discord] Running quest: %s (%.0f/%.0f min elapsed)", quest.Name(),
		alreadyElapsed.Minutes(), totalDur.Minutes())

	start := time.Now()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if err := c.Heartbeat(ctx, quest.ID, isStream); err != nil {
			return fmt.Errorf("heartbeat: %w", err)
		}

		elapsed := alreadyElapsed + time.Since(start)
		if onProgress != nil {
			onProgress(elapsed, totalDur)
		}

		if elapsed >= totalDur {
			break
		}

		interval := HeartbeatInterval()
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(interval):
		}
	}

	c.logf("[Discord] Quest %s done, claiming reward...", quest.Name())
	return c.Claim(ctx, quest.ID)
}

