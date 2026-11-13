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
	"regexp"
	"strconv"
	"strings"
	"time"
)

var apiBase = "https://discord.com/api/v9/"

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
	ApplicationID   string  `json:"application_id"`
	Duration        int     `json:"stream_duration_requirement_minutes"`
	DurationSeconds float64 `json:"duration_seconds"`
	Target          float64 `json:"target"`
	TaskType        string  `json:"type"` // PLAY_ON_DESKTOP, STREAM_ON_DESKTOP, WATCH_VIDEO, etc.
}

// TaskV2 represents an objective in task_config_v2.
type TaskV2 struct {
	Target          float64 `json:"target"` // Target in seconds
	TargetSeconds   float64 `json:"target_seconds"`
	Duration        float64 `json:"duration"`
	DurationSeconds float64 `json:"duration_seconds"`
	Applications    []struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"applications"`
}

func (t TaskV2) GetTarget() float64 {
	if t.Target > 0 {
		return t.Target
	}
	if t.TargetSeconds > 0 {
		return t.TargetSeconds
	}
	if t.DurationSeconds > 0 {
		return t.DurationSeconds
	}
	if t.Duration > 0 {
		return t.Duration
	}
	return 0
}

// TaskConfigV2 represents task_config_v2 structure.
type TaskConfigV2 struct {
	Tasks map[string]TaskV2 `json:"tasks"`
}

// QuestRewardItem holds an individual reward item in reward_config.
type QuestRewardItem struct {
	Type       int    `json:"type"`
	SkuID      string `json:"sku_id"`
	Asset      string `json:"asset"`
	Name       string `json:"name"`
	RewardName string `json:"reward_name"`
	Messages   struct {
		Name       string `json:"name"`
		RewardName string `json:"reward_name"`
	} `json:"messages"`
}

// QuestReward contains reward description and details.
type QuestReward struct {
	Name       string            `json:"name"`
	RewardName string            `json:"reward_name"`
	SkuID      string            `json:"sku_id"`
	Messages   struct {
		Completed  string `json:"completed"`
		Name       string `json:"name"`
		RewardName string `json:"reward_name"`
	} `json:"messages"`
	Rewards    []QuestRewardItem `json:"rewards"`
	RewardList []QuestRewardItem `json:"reward_list"`
	OrbsReward *struct {
		Amount int `json:"amount"`
	} `json:"orbs_reward"`
	Orbs *struct {
		Amount int `json:"amount"`
	} `json:"orbs"`
}

// QuestConfig holds quest metadata.
type QuestConfig struct {
	ExpiresAt     string          `json:"expires_at"`
	StartsAt      string          `json:"starts_at"`
	Description   string          `json:"description"`
	TaskConfig    QuestTaskConfig `json:"quest_task_config"`
	TaskConfigV2  *TaskConfigV2   `json:"task_config_v2"`
	RewardConfig  QuestReward     `json:"reward_config"`
	VideoMetadata any             `json:"video_metadata"`
	VideoID       string          `json:"video_id"`
	Assets        struct {
		Title string `json:"title"`
		Video string `json:"video"`
	} `json:"assets"`
	Messages struct {
		QuestName     string `json:"quest_name"`
		GameTitle     string `json:"game_title"`
		GamePublisher string `json:"game_publisher"`
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
	RewardName    string  `json:"reward_name"`    // Short reward badge name, e.g. "840 Orbs"
	GameTitle     string  `json:"game_title"`     // Human readable game title e.g. "Arknights: Endfield"
	IsVideo       bool    `json:"is_video"`
	IsClaimed     bool    `json:"is_claimed"`  // already claimed (shown in completed section)
	IsExpired     bool    `json:"is_expired"`  // quest is past its expiry date
}

// Name returns the human-readable quest or game name.
func (q Quest) Name() string {
	if q.Config.Messages.GameTitle != "" {
		return q.Config.Messages.GameTitle
	}
	if q.Config.Messages.QuestName != "" {
		return q.Config.Messages.QuestName
	}
	return "Quest #" + q.ID
}

var (
	orbsRegex    = regexp.MustCompile(`(?i)(?:[«"']|\s|^)(\d+[\s\x{00a0}]*orbs?|[\d,]+\s*orbs?)(?:[»"']|\s|$)`)
	keywordRegex = regexp.MustCompile(`(?i)(?:[«"']|\s|^)((?:[A-Za-zА-Яа-я0-9-]+\s+){0,3}(?:Decoration|Blueprint|Profile Effect|Avatar Decoration|Badge|Skin|Bundle|Чертеж|Чертёж|Украшение|Декорация|Эффект))(?:[»"']|\s|$)`)
	verbsRegex   = regexp.MustCompile(`^(?i)(?:earn|claim|get|unlock|receive|win|watch and earn|watch and claim|получить|забрать|получите|заберите|награда|награду)\s+`)
)

func cleanRewardString(s string) string {
	s = strings.Trim(s, " \t\r\n«»\"'“”")
	s = verbsRegex.ReplaceAllString(s, "")
	s = strings.Trim(s, " \t\r\n«»\"'“”")
	return s
}

func (q *Quest) extractReward() string {
	rc := q.Config.RewardConfig

	// 1. Direct names on rc
	candidates := []string{
		rc.Messages.RewardName,
		rc.Messages.Name,
		rc.RewardName,
		rc.Name,
	}
	for _, c := range candidates {
		if strings.TrimSpace(c) != "" {
			return strings.TrimSpace(c)
		}
	}

	// 2. Rewards and RewardList arrays
	itemList := append([]QuestRewardItem{}, rc.Rewards...)
	itemList = append(itemList, rc.RewardList...)
	for _, r := range itemList {
		itemCandidates := []string{
			r.Messages.RewardName,
			r.Messages.Name,
			r.RewardName,
			r.Name,
		}
		for _, c := range itemCandidates {
			if strings.TrimSpace(c) != "" {
				return strings.TrimSpace(c)
			}
		}
	}

	// 3. Orbs objects
	if rc.OrbsReward != nil && rc.OrbsReward.Amount > 0 {
		return fmt.Sprintf("%d Orbs", rc.OrbsReward.Amount)
	}
	if rc.Orbs != nil && rc.Orbs.Amount > 0 {
		return fmt.Sprintf("%d Orbs", rc.Orbs.Amount)
	}

	// 4. Parse from text descriptions
	textPool := []string{
		rc.Messages.RewardName,
		rc.Messages.Name,
		rc.RewardName,
		rc.Name,
		rc.Messages.Completed,
		q.Config.Description,
		q.Config.Messages.QuestName,
		q.Config.Messages.GameTitle,
		q.Config.Messages.GamePublisher,
		q.Config.Assets.Title,
	}
	for _, txt := range textPool {
		if txt == "" {
			continue
		}
		if sub := orbsRegex.FindStringSubmatch(txt); len(sub) > 1 {
			val := cleanRewardString(sub[1])
			if val != "" {
				parts := strings.Fields(val)
				if len(parts) >= 2 && strings.EqualFold(parts[1], "orbs") {
					return parts[0] + " Orbs"
				}
				return val
			}
		}
		if sub := keywordRegex.FindStringSubmatch(txt); len(sub) > 1 {
			val := cleanRewardString(sub[1])
			if val != "" {
				return val
			}
		}
	}

	// 5. Fallback short completed message
	if rc.Messages.Completed != "" && len(rc.Messages.Completed) <= 45 {
		return strings.TrimSpace(rc.Messages.Completed)
	}

	return ""
}

// PopulateMetrics calculates SecondsDone, SecondsTarget, ProgressRatio and TaskName.
func (q *Quest) PopulateMetrics() {
	gt := q.Config.Messages.GameTitle
	if gt == "" {
		gt = q.Config.Messages.GamePublisher
	}
	q.GameTitle = gt

	taskName := ""
	var target float64 = 0

	// 1. Determine task from task_config_v2 if available
	if q.Config.TaskConfigV2 != nil && len(q.Config.TaskConfigV2.Tasks) > 0 {
		candidates := []string{
			"WATCH_VIDEO",
			"WATCH_VIDEO_DESKTOP",
			"WATCH_VIDEO_ON_DESKTOP",
			"WATCH_VIDEO_ON_MOBILE",
			"WATCH_ON_MOBILE",
			"VIDEO_PLAY",
			"VIEW_VIDEO",
			"VIDEO",
			"PLAY_ON_DESKTOP",
			"STREAM_ON_DESKTOP",
			"PLAY_ACTIVITY",
		}
		for _, c := range candidates {
			for k, t := range q.Config.TaskConfigV2.Tasks {
				if strings.EqualFold(k, c) {
					taskName = k
					target = t.GetTarget()
					break
				}
			}
			if taskName != "" {
				break
			}
		}
		if taskName == "" {
			for k, t := range q.Config.TaskConfigV2.Tasks {
				taskName = k
				target = t.GetTarget()
				break
			}
		}
	}

	// 2. Fallback to task_config (v1)
	if taskName == "" {
		taskName = q.Config.TaskConfig.TaskType
	}

	upperTask := strings.ToUpper(taskName)
	isVideo := strings.Contains(upperTask, "VIDEO") || strings.Contains(upperTask, "WATCH") || strings.Contains(upperTask, "VIEW")

	// Also check if any task in task_config_v2 is video
	if !isVideo && q.Config.TaskConfigV2 != nil {
		for k, t := range q.Config.TaskConfigV2.Tasks {
			uk := strings.ToUpper(k)
			if strings.Contains(uk, "VIDEO") || strings.Contains(uk, "WATCH") || strings.Contains(uk, "VIEW") {
				isVideo = true
				if taskName == "" || (!strings.Contains(strings.ToUpper(taskName), "VIDEO") && !strings.Contains(strings.ToUpper(taskName), "WATCH") && !strings.Contains(strings.ToUpper(taskName), "VIEW")) {
					taskName = k
					if t.GetTarget() > 0 {
						target = t.GetTarget()
					}
				}
				break
			}
		}
	}
	if !isVideo && (q.Config.VideoMetadata != nil || q.Config.VideoID != "" || q.Config.Assets.Video != "" || strings.Contains(strings.ToUpper(q.Config.TaskConfig.TaskType), "VIDEO") || strings.Contains(strings.ToUpper(q.Config.TaskConfig.TaskType), "WATCH")) {
		isVideo = true
	}

	if target <= 0 {
		if q.Config.TaskConfig.Target > 0 {
			target = q.Config.TaskConfig.Target
		} else if q.Config.TaskConfig.DurationSeconds > 0 {
			target = q.Config.TaskConfig.DurationSeconds
		} else if q.Config.TaskConfig.Duration > 0 {
			target = float64(q.Config.TaskConfig.Duration * 60)
		} else if isVideo {
			target = 30 // 30 sec default for video
		} else {
			target = 900 // 15 minutes default
		}
	}

	q.TaskName = taskName
	q.SecondsTarget = target
	q.IsVideo = isVideo

	// Mark claimed status
	if q.UserStatus != nil && q.UserStatus.ClaimedAt != "" {
		q.IsClaimed = true
	}

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

	// 4. Determine RewardName
	q.RewardName = q.extractReward()
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

	for attempt := 0; attempt < 8; attempt++ {
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
			// Add 1-2s jitter to avoid colliding on exact rate limit windows
			wait += time.Duration(1000+rand.IntN(1000)) * time.Millisecond

			if wait > 60*time.Second {
				mins := int(wait.Minutes())
				msg := fmt.Sprintf("Discord rate limit cooldown active: please wait %d min before trying again", mins)
				c.logf("[Discord] ⚠️ %s", msg)
				return nil, fmt.Errorf("%s", msg)
			}
			c.logf("[Discord] Rate limited (attempt %d/8), waiting %v...", attempt+1, wait.Round(time.Millisecond))
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

func parseTimeFlexible(s string) (time.Time, bool) {
	if s == "" {
		return time.Time{}, false
	}
	formats := []string{
		time.RFC3339,
		time.RFC3339Nano,
		"2006-01-02T15:04:05Z07:00",
		"2006-01-02T15:04:05Z",
		"2006-01-02T15:04:05",
		"2006-01-02 15:04:05",
		"2006-01-02",
	}
	for _, f := range formats {
		if t, err := time.Parse(f, s); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

// filterQuests removes genuinely expired quests and marks claimed ones.
// It returns (active, claimed) slices.
func filterQuests(all []Quest) (active []Quest, claimed []Quest) {
	now := time.Now()
	for _, q := range all {
		// Video quests (GTA VI, KOF, Monster Hunter, Anno 117) are never excluded by expiry filter
		if q.IsVideo {
			if q.IsClaimed {
				claimed = append(claimed, q)
			} else {
				active = append(active, q)
			}
			continue
		}

		// Only filter out quests strictly in the past with a 2-hour grace window.
		// If date cannot be parsed or timezone is uncertain, do NOT filter the quest out.
		if q.Config.ExpiresAt != "" {
			if exp, ok := parseTimeFlexible(q.Config.ExpiresAt); ok {
				if now.After(exp.Add(2 * time.Hour)) {
					q.IsExpired = true
					continue // skip expired quests
				}
			}
		}
		// Separate claimed quests
		if q.IsClaimed {
			claimed = append(claimed, q)
		} else {
			active = append(active, q)
		}
	}
	return active, claimed
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

// FetchQuests retrieves and filters available quests for the authenticated user.
// Active (unclaimed, non-expired) quests come first; claimed quests are appended at the end (marked IsClaimed=true).
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

	all, err := ParseQuests(b)
	if err != nil {
		return nil, fmt.Errorf("discord: parse quests: %w", err)
	}

	// Filter: separate active from claimed; drop expired entirely
	active, claimed := filterQuests(all)

	// Combine: active first, then claimed (for display in "completed" section)
	result := make([]Quest, 0, len(active)+len(claimed))
	result = append(result, active...)
	result = append(result, claimed...)

	return result, nil
}

// Enroll accepts a quest (with realistic delay).
func (c *Client) Enroll(ctx context.Context, questID string) error {
	jitter := time.Duration(1000+rand.IntN(2000)) * time.Millisecond
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(jitter):
	}

	body := map[string]any{
		"location": 1,
		"platform": 4, // 4 = PC / DESKTOP in Discord API
	}
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

// DiscordUser represents profile information from /users/@me.
type DiscordUser struct {
	ID            string `json:"id"`
	Username      string `json:"username"`
	GlobalName    string `json:"global_name"`
	Avatar        string `json:"avatar"`
	Discriminator string `json:"discriminator"`
}

// DisplayName returns global name if set, otherwise username.
func (u *DiscordUser) DisplayName() string {
	if u.GlobalName != "" {
		return u.GlobalName
	}
	return u.Username
}

// AvatarURL returns the user's avatar image URL.
func (u *DiscordUser) AvatarURL() string {
	if u.Avatar != "" {
		return fmt.Sprintf("https://cdn.discordapp.com/avatars/%s/%s.png", u.ID, u.Avatar)
	}
	return "https://cdn.discordapp.com/embed/avatars/0.png"
}

// FetchUser fetches the current user's profile from Discord.
func (c *Client) FetchUser(ctx context.Context) (*DiscordUser, error) {
	resp, err := c.doReq(ctx, "GET", "users/@me", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("discord: get user: HTTP %d", resp.StatusCode)
	}
	var u DiscordUser
	if err := json.NewDecoder(resp.Body).Decode(&u); err != nil {
		return nil, err
	}
	return &u, nil
}

// HeartbeatInterval returns 32-36 seconds for realistic timing avoiding rate limits.
func HeartbeatInterval() time.Duration {
	return 32*time.Second + time.Duration(rand.IntN(5))*time.Second // 32-36s
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
		c.logf("[Discord] Quest %s already completed (%.0f/%.0f s). Please claim your reward in Discord client.", quest.Name(), alreadyDone, totalSec)
		return nil
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
		c.logf("[Discord] Quest %s completed (100%%)! Please claim your reward in Discord client.", quest.Name())
		return nil
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

	c.logf("[Discord] Quest %s completed (100%%)! Please claim your reward in Discord client.", quest.Name())
	return nil
}
