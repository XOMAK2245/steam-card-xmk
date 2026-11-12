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
	"strings"
	"time"
)

const apiBase = "https://discord.com/api/v9/"

// ErrUnauthorized is returned when the Discord token is invalid or expired.
var ErrUnauthorized = errors.New("discord: invalid token (401)")

// ---------- Quest data types ----------

// TaskProgress holds task-specific progress metrics.
type TaskProgress struct {
	Value float64 `json:"value"`
}

// QuestUserStatus tracks user progress on a quest.
type QuestUserStatus struct {
	EnrolledAt            string                  `json:"enrolled_at"`
	CompletedAt           string                  `json:"completed_at"`
	ClaimedAt             string                  `json:"claimed_at"`
	LastStreamHeartbeatAt string                  `json:"last_stream_heartbeat_at"`
	StreamProgressSeconds float64                 `json:"stream_progress_seconds"`
	CompletedRatio        float64                 `json:"completed_ratio"`
	Progress              map[string]TaskProgress `json:"progress"`
}

// QuestTaskConfig describes what the user must do for a quest (v1 format).
type QuestTaskConfig struct {
	ApplicationID string `json:"application_id"`
	Duration      int    `json:"stream_duration_requirement_minutes"`
	TaskType      string `json:"type"` // PLAY_ON_DESKTOP, STREAM_ON_DESKTOP, WATCH_VIDEO, etc.
}

// TaskV2 represents an objective in task_config_v2.
type TaskV2 struct {
	Target       float64 `json:"target"` // Target in seconds
	Applications []struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"applications"`
}

// TaskConfigV2 represents task_config_v2 structure.
type TaskConfigV2 struct {
	Tasks map[string]TaskV2 `json:"tasks"`
}

// QuestReward contains reward description.
type QuestReward struct {
	Messages struct {
		Completed string `json:"completed"`
	} `json:"messages"`
}

// QuestConfig holds quest metadata.
type QuestConfig struct {
	ExpiresAt    string          `json:"expires_at"`
	TaskConfig   QuestTaskConfig `json:"quest_task_config"`
	TaskConfigV2 *TaskConfigV2   `json:"task_config_v2"`
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

	// Calculated helpers computed for easy frontend consumption & worker logic:
	SecondsDone   float64 `json:"seconds_done"`
	SecondsTarget float64 `json:"seconds_target"`
	ProgressRatio float64 `json:"progress_ratio"` // 0.0 to 1.0
	TaskName      string  `json:"task_name"`      // WATCH_VIDEO, PLAY_ON_DESKTOP, STREAM_ON_DESKTOP, etc.
	IsVideo       bool    `json:"is_video"`
}

// Name returns the human-readable quest name.
func (q Quest) Name() string { return q.Config.Messages.QuestName }

// PopulateMetrics calculates SecondsDone, SecondsTarget, ProgressRatio and TaskName.
func (q *Quest) PopulateMetrics() {
	taskName := ""
	var target float64 = 0

	// 1. Determine task from task_config_v2 if available
	if q.Config.TaskConfigV2 != nil && len(q.Config.TaskConfigV2.Tasks) > 0 {
		candidates := []string{"WATCH_VIDEO", "WATCH_VIDEO_ON_MOBILE", "PLAY_ON_DESKTOP", "STREAM_ON_DESKTOP", "PLAY_ACTIVITY"}
		for _, c := range candidates {
			if t, ok := q.Config.TaskConfigV2.Tasks[c]; ok {
				taskName = c
				target = t.Target
				break
			}
		}
		if taskName == "" {
			for k, t := range q.Config.TaskConfigV2.Tasks {
				taskName = k
				target = t.Target
				break
			}
		}
	}

	// 2. Fallback to task_config (v1)
	if taskName == "" {
		taskName = q.Config.TaskConfig.TaskType
	}
	if target <= 0 {
		if q.Config.TaskConfig.Duration > 0 {
			target = float64(q.Config.TaskConfig.Duration * 60)
		} else {
			target = 900 // 15 minutes default
		}
	}

	q.TaskName = taskName
	q.SecondsTarget = target
	q.IsVideo = strings.Contains(taskName, "WATCH_VIDEO")

	// 3. Determine SecondsDone
	var done float64 = 0
	if q.UserStatus != nil {
		if q.UserStatus.Progress != nil {
			if p, ok := q.UserStatus.Progress[taskName]; ok && p.Value > 0 {
				done = p.Value
			} else {
				// Search any positive progress
				for _, p := range q.UserStatus.Progress {
					if p.Value > done {
						done = p.Value
					}
				}
			}
		}
		if done <= 0 && q.UserStatus.StreamProgressSeconds > 0 {
			done = q.UserStatus.StreamProgressSeconds
		}
		if done <= 0 && q.UserStatus.CompletedRatio > 0 && target > 0 {
			done = q.UserStatus.CompletedRatio * target
		}
	}
	q.SecondsDone = done

	if target > 0 {
		q.ProgressRatio = done / target
		if q.ProgressRatio > 1.0 {
			q.ProgressRatio = 1.0
		}
	}
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
			if wait > 60*time.Second {
				mins := int(wait.Minutes())
				msg := fmt.Sprintf("Discord rate limit cooldown active: please wait %d min before trying again", mins)
				c.logf("[Discord] ⚠️ %s", msg)
				return nil, fmt.Errorf("%s", msg)
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
	var quests []Quest

	// 1. Try Discord's standard object format: {"quests": [...]}
	var res questsResponse
	if err := json.Unmarshal(data, &res); err == nil && res.Quests != nil {
		quests = res.Quests
	} else {
		// 2. Try raw array format: [...]
		var list []Quest
		if err := json.Unmarshal(data, &list); err == nil {
			quests = list
		}
	}

	if quests == nil {
		return nil, fmt.Errorf("unexpected quests JSON structure")
	}

	for i := range quests {
		quests[i].PopulateMetrics()
	}
	return quests, nil
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

	body := map[string]any{"location": 1}
	resp, err := c.doReq(ctx, "POST", "quests/"+questID+"/enroll", body)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return fmt.Errorf("discord: enroll %s: HTTP %d: %s", questID, resp.StatusCode, string(b))
	}
	return nil
}

// Heartbeat sends a progress heartbeat for a game/stream quest.
// terminal is false during progress, and true on the final finishing beat.
func (c *Client) Heartbeat(ctx context.Context, questID string, terminal bool) error {
	body := map[string]any{"terminal": terminal}
	resp, err := c.doReq(ctx, "POST", "quests/"+questID+"/heartbeat", body)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return fmt.Errorf("discord: heartbeat %s: HTTP %d: %s", questID, resp.StatusCode, string(b))
	}
	return nil
}

// VideoProgress reports video watch timestamp progress for video quests.
func (c *Client) VideoProgress(ctx context.Context, questID string, timestamp float64) error {
	body := map[string]any{"timestamp": timestamp}
	resp, err := c.doReq(ctx, "POST", "quests/"+questID+"/video-progress", body)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return fmt.Errorf("discord: video-progress %s: HTTP %d: %s", questID, resp.StatusCode, string(b))
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

	body := map[string]any{"location": 1}
	resp, err := c.doReq(ctx, "POST", "quests/"+questID+"/claim", body)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return fmt.Errorf("discord: claim %s: HTTP %d: %s", questID, resp.StatusCode, string(b))
	}
	return nil
}

// HeartbeatInterval returns 30 seconds ± random jitter for realistic timing.
func HeartbeatInterval() time.Duration {
	return 30*time.Second + time.Duration(rand.IntN(7)-3)*time.Second // 27-33s
}

// RunQuest runs a quest to completion: enroll (if needed) → progress loop (video or heartbeat) → claim.
// onProgress is called periodically with (elapsedSeconds, totalSeconds).
func (c *Client) RunQuest(ctx context.Context, quest Quest, onProgress func(elapsed, total float64)) error {
	quest.PopulateMetrics()
	totalSec := quest.SecondsTarget
	if totalSec <= 0 {
		totalSec = 900 // 15 min default
	}

	// 1. Enroll if not already enrolled
	if quest.UserStatus != nil && quest.UserStatus.EnrolledAt != "" {
		c.logf("[Discord] Quest %s already enrolled", quest.Name())
	} else {
		c.logf("[Discord] Enrolling in quest: %s", quest.Name())
		if err := c.Enroll(ctx, quest.ID); err != nil {
			return fmt.Errorf("enroll: %w", err)
		}
	}

	alreadyDone := quest.SecondsDone
	if alreadyDone >= totalSec {
		c.logf("[Discord] Quest %s already completed (%.0f/%.0f s), claiming reward...", quest.Name(), alreadyDone, totalSec)
		return c.Claim(ctx, quest.ID)
	}

	// 2. Video quest path
	if quest.IsVideo {
		c.logf("[Discord] Running video quest: %s (%.0f/%.0f s)", quest.Name(), alreadyDone, totalSec)
		current := alreadyDone
		for current < totalSec {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}

			// Natural watch step: 5 to 10 seconds
			step := float64(5 + rand.IntN(6))
			current += step
			if current > totalSec {
				current = totalSec
			}

			if err := c.VideoProgress(ctx, quest.ID, current); err != nil {
				return fmt.Errorf("video-progress: %w", err)
			}
			if onProgress != nil {
				onProgress(current, totalSec)
			}
			if current >= totalSec {
				break
			}

			// Pause naturally matching step with slight jitter
			wait := time.Duration(step*1000) * time.Millisecond
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(wait):
			}
		}
		c.logf("[Discord] Video quest %s watched, claiming reward...", quest.Name())
		return c.Claim(ctx, quest.ID)
	}

	// 3. Game/Stream heartbeat loop
	c.logf("[Discord] Running play/stream quest: %s (%.1f/%.1f min elapsed)", quest.Name(), alreadyDone/60, totalSec/60)
	start := time.Now()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		elapsed := alreadyDone + time.Since(start).Seconds()
		terminal := elapsed >= totalSec

		if err := c.Heartbeat(ctx, quest.ID, terminal); err != nil {
			return fmt.Errorf("heartbeat: %w", err)
		}
		if onProgress != nil {
			onProgress(elapsed, totalSec)
		}
		if terminal {
			break
		}

		interval := HeartbeatInterval()
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(interval):
		}
	}

	c.logf("[Discord] Quest %s finished, claiming reward...", quest.Name())
	return c.Claim(ctx, quest.ID)
}
