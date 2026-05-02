// Package api hosts HTTP handlers exposed by the bot.
//
// Webhook layout (single ingress, per-source path, per-source auth):
//
//	POST /webhook/gha     ← our own GitHub Actions deploy notifier
//	                        auth: Bearer GHA_WEBHOOK_TOKEN
//	POST /webhook/github  ← GitHub repo events (release / push / etc.)
//	                        auth: HMAC-SHA256 GITHUB_WEBHOOK_SECRET
//
// Shared middleware (RegisterWebhooks): reads the body once, increments
// per-source metrics, dedups on a stable header (X-GitHub-Delivery for
// GitHub; X-Webhook-Id we mint for our own GHA payload). Each handler
// then verifies its own auth on the cached body.
package api

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/gin-gonic/gin"
	"github.com/open-xiv/memo-discord-bot/bot"
	"github.com/open-xiv/memo-discord-bot/flow"
	"github.com/open-xiv/memo-discord-bot/metrics"
	"github.com/open-xiv/memo-discord-bot/service/notifier"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"
)

const (
	// dedupTTL — how long we remember a webhook delivery ID. GitHub may
	// retry within minutes; 24h is well past any realistic retry window
	// and bounds Redis growth.
	dedupTTL = 24 * time.Hour

	// maxBodyBytes — webhook payloads can be large (e.g. push events with
	// hundreds of commits) but we don't need that much. 1 MiB is generous
	// while still rejecting obvious abuse.
	maxBodyBytes = 1 << 20
)

// RegisterWebhooks wires /webhook/* routes onto the given gin engine.
func RegisterWebhooks(r *gin.Engine) {
	g := r.Group("/webhook", webhookMiddleware())
	g.POST("/gha", handleGHA)
	g.POST("/github", handleGitHub)
}

// webhookMiddleware reads the body once + caches it, dedups by delivery ID,
// and emits a per-source / per-event status metric on the way out.
func webhookMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		source := lastPathSegment(c.FullPath()) // "gha" / "github" / ""
		body, err := io.ReadAll(io.LimitReader(c.Request.Body, maxBodyBytes))
		_ = c.Request.Body.Close()
		if err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "read body: " + err.Error()})
			metrics.WebhooksTotal.WithLabelValues(source, "", "read_error").Inc()
			return
		}
		c.Set("webhook.body", body)
		// Restore body so any downstream c.Request.Body access still works.
		c.Request.Body = io.NopCloser(bytes.NewReader(body))

		deliveryID := firstNonEmpty(
			c.GetHeader("X-GitHub-Delivery"),
			c.GetHeader("X-Webhook-Id"),
		)
		if deliveryID != "" && flow.Redis != nil {
			key := "bot:webhook:dedup:" + source + ":" + deliveryID
			ok, rerr := flow.Redis.SetNX(c, key, "1", dedupTTL).Result()
			if rerr == nil && !ok {
				log.Info().Str("source", source).Str("delivery", deliveryID).
					Msg("webhook duplicate, skipping")
				metrics.WebhooksTotal.WithLabelValues(source, "", "duplicate").Inc()
				c.AbortWithStatusJSON(http.StatusOK, gin.H{"status": "duplicate"})
				return
			}
			if rerr != nil && !errors.Is(rerr, redis.Nil) {
				// dedup failure is not fatal — fall through and process
				log.Warn().Err(rerr).Msg("dedup setnx failed; processing anyway")
			}
		}
		c.Set("webhook.delivery_id", deliveryID)
		c.Set("webhook.source", source)

		c.Next()

		// status label is set by handler via context, otherwise infer from code
		status, _ := c.Get("webhook.status")
		statusStr, _ := status.(string)
		if statusStr == "" {
			if c.Writer.Status() >= 200 && c.Writer.Status() < 300 {
				statusStr = "ok"
			} else {
				statusStr = "error"
			}
		}
		event, _ := c.Get("webhook.event")
		eventStr, _ := event.(string)
		metrics.WebhooksTotal.WithLabelValues(source, eventStr, statusStr).Inc()
	}
}

// ----------------------------------------------------------------------
// /webhook/gha — our own GitHub Actions deploy notifier
// ----------------------------------------------------------------------

type ghaPayload struct {
	Service string `json:"service"`
	Tag     string `json:"tag"`
	Cluster string `json:"cluster"`
	Status  string `json:"status"` // "success" | "failure"
	Build   string `json:"build"`  // result of build job
	Deploy  string `json:"deploy"` // result of deploy job
	Commit  string `json:"commit"`
	Title   string `json:"title"`
	Actor   string `json:"actor"`
	RunURL  string `json:"run_url"`
	Ref     string `json:"ref"`
}

func handleGHA(c *gin.Context) {
	expected := os.Getenv("GHA_WEBHOOK_TOKEN")
	if expected == "" {
		log.Error().Msg("GHA_WEBHOOK_TOKEN not configured")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "not configured"})
		return
	}
	got := strings.TrimPrefix(c.GetHeader("Authorization"), "Bearer ")
	if !hmac.Equal([]byte(got), []byte(expected)) {
		c.Set("webhook.status", "unauthorized")
		c.JSON(http.StatusUnauthorized, gin.H{"error": "bad token"})
		return
	}

	body, _ := c.Get("webhook.body")
	var p ghaPayload
	if err := json.Unmarshal(body.([]byte), &p); err != nil {
		c.Set("webhook.status", "bad_request")
		c.JSON(http.StatusBadRequest, gin.H{"error": "bad json: " + err.Error()})
		return
	}
	c.Set("webhook.event", "deploy")

	// Build the embed.
	emoji := "✅"
	color := notifier.ColorSuccess
	verb := "deployed"
	if p.Status != "success" {
		emoji = "❌"
		color = notifier.ColorFailure
		// Be specific about which leg failed when we can.
		switch {
		case p.Build != "" && p.Build != "success":
			verb = fmt.Sprintf("build failed (%s)", p.Build)
		case p.Deploy != "" && p.Deploy != "success":
			verb = fmt.Sprintf("deploy failed (%s)", p.Deploy)
		default:
			verb = "deploy failed"
		}
	}

	cluster := p.Cluster
	if cluster == "" {
		cluster = "?"
	}
	tag := p.Tag
	if tag == "" {
		tag = "(no image)"
	}

	title := fmt.Sprintf("%s %s %s", emoji, p.Service, verb)
	desc := fmt.Sprintf("`%s` · cluster: `%s`", tag, cluster)
	if p.Title != "" {
		desc += "\n" + truncate(p.Title, 200)
	}

	fields := []*discordgo.MessageEmbedField{}
	if p.Commit != "" {
		shortSHA := p.Commit
		if len(shortSHA) > 7 {
			shortSHA = shortSHA[:7]
		}
		fields = append(fields, &discordgo.MessageEmbedField{
			Name: "Commit", Value: shortSHA, Inline: true,
		})
	}
	if p.Actor != "" {
		fields = append(fields, &discordgo.MessageEmbedField{
			Name: "Author", Value: p.Actor, Inline: true,
		})
	}
	if p.Ref != "" {
		fields = append(fields, &discordgo.MessageEmbedField{
			Name: "Ref", Value: p.Ref, Inline: true,
		})
	}
	if p.RunURL != "" {
		fields = append(fields, &discordgo.MessageEmbedField{
			Name: "Run", Value: "[GitHub Actions](" + p.RunURL + ")", Inline: false,
		})
	}

	embed := &discordgo.MessageEmbed{
		Title:       title,
		Description: desc,
		Color:       color,
		Fields:      fields,
		Timestamp:   time.Now().UTC().Format(time.RFC3339),
	}

	if _, err := notifier.SendEmbed(bot.DevChannelID, embed); err != nil {
		log.Error().Err(err).Msg("send gha embed failed")
		c.Set("webhook.status", "send_error")
		c.JSON(http.StatusBadGateway, gin.H{"error": "send failed"})
		return
	}
	log.Info().Str("service", p.Service).Str("tag", tag).Str("status", p.Status).
		Msg("gha webhook posted")
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// ----------------------------------------------------------------------
// /webhook/github — GitHub repo events (HMAC verified)
// ----------------------------------------------------------------------

func handleGitHub(c *gin.Context) {
	secret := os.Getenv("GITHUB_WEBHOOK_SECRET")
	if secret == "" {
		log.Error().Msg("GITHUB_WEBHOOK_SECRET not configured")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "not configured"})
		return
	}

	body, _ := c.Get("webhook.body")
	bodyBytes, _ := body.([]byte)
	if !verifyGitHubSignature(c.GetHeader("X-Hub-Signature-256"), secret, bodyBytes) {
		c.Set("webhook.status", "unauthorized")
		c.JSON(http.StatusUnauthorized, gin.H{"error": "signature mismatch"})
		return
	}

	event := c.GetHeader("X-GitHub-Event")
	c.Set("webhook.event", event)

	// Always 200 the ping — GitHub uses this to confirm reachability.
	if event == "ping" {
		log.Info().Msg("github webhook ping received")
		c.JSON(http.StatusOK, gin.H{"status": "pong"})
		return
	}

	// We only render a small subset of events. Everything else gets a
	// counted "ignored" without any Discord noise.
	var (
		embed     *discordgo.MessageEmbed
		shouldPost = true
	)
	switch event {
	case "release":
		embed = renderRelease(bodyBytes)
	case "workflow_run":
		// only report failures here — successes flow through /webhook/gha
		// from our own notify job, where we have richer context
		embed, shouldPost = renderWorkflowFailure(bodyBytes)
	default:
		shouldPost = false
		c.Set("webhook.status", "ignored")
	}

	if !shouldPost || embed == nil {
		c.JSON(http.StatusOK, gin.H{"status": "ignored", "event": event})
		return
	}

	if _, err := notifier.SendEmbed(bot.DevChannelID, embed); err != nil {
		log.Error().Err(err).Msg("send github embed failed")
		c.Set("webhook.status", "send_error")
		c.JSON(http.StatusBadGateway, gin.H{"error": "send failed"})
		return
	}
	log.Info().Str("event", event).Msg("github webhook posted")
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func verifyGitHubSignature(header, secret string, body []byte) bool {
	const prefix = "sha256="
	if !strings.HasPrefix(header, prefix) {
		return false
	}
	want, err := hex.DecodeString(header[len(prefix):])
	if err != nil {
		return false
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	got := mac.Sum(nil)
	return hmac.Equal(got, want)
}

// Minimal partial structs — we only read what we render.

type ghRelease struct {
	Action  string `json:"action"`
	Release struct {
		Name        string `json:"name"`
		TagName     string `json:"tag_name"`
		HTMLURL     string `json:"html_url"`
		Body        string `json:"body"`
		Prerelease  bool   `json:"prerelease"`
	} `json:"release"`
	Repository struct {
		FullName string `json:"full_name"`
	} `json:"repository"`
	Sender struct {
		Login string `json:"login"`
	} `json:"sender"`
}

func renderRelease(body []byte) *discordgo.MessageEmbed {
	var p ghRelease
	if err := json.Unmarshal(body, &p); err != nil {
		return nil
	}
	if p.Action != "published" {
		return nil
	}
	title := fmt.Sprintf("🏷️ %s · %s", p.Repository.FullName, p.Release.TagName)
	if p.Release.Name != "" && p.Release.Name != p.Release.TagName {
		title += " — " + p.Release.Name
	}
	desc := truncate(p.Release.Body, 1500)
	if desc == "" {
		desc = "_(no release notes)_"
	}
	color := notifier.ColorInfo
	if p.Release.Prerelease {
		color = notifier.ColorWarn
	}
	return &discordgo.MessageEmbed{
		Title:       title,
		URL:         p.Release.HTMLURL,
		Description: desc,
		Color:       color,
		Footer:      &discordgo.MessageEmbedFooter{Text: "release · by " + p.Sender.Login},
		Timestamp:   time.Now().UTC().Format(time.RFC3339),
	}
}

type ghWorkflowRun struct {
	Action      string `json:"action"`
	WorkflowRun struct {
		Name       string `json:"name"`
		Status     string `json:"status"`     // "completed"
		Conclusion string `json:"conclusion"` // "success" / "failure" / ...
		HTMLURL    string `json:"html_url"`
		HeadBranch string `json:"head_branch"`
		HeadSHA    string `json:"head_sha"`
		Actor      struct {
			Login string `json:"login"`
		} `json:"actor"`
	} `json:"workflow_run"`
	Repository struct {
		FullName string `json:"full_name"`
	} `json:"repository"`
}

func renderWorkflowFailure(body []byte) (*discordgo.MessageEmbed, bool) {
	var p ghWorkflowRun
	if err := json.Unmarshal(body, &p); err != nil {
		return nil, false
	}
	if p.Action != "completed" || p.WorkflowRun.Conclusion == "success" {
		return nil, false
	}
	if p.WorkflowRun.Conclusion == "" || p.WorkflowRun.Conclusion == "skipped" || p.WorkflowRun.Conclusion == "cancelled" {
		return nil, false
	}
	short := p.WorkflowRun.HeadSHA
	if len(short) > 7 {
		short = short[:7]
	}
	embed := &discordgo.MessageEmbed{
		Title:       fmt.Sprintf("❌ %s · workflow failed", p.Repository.FullName),
		URL:         p.WorkflowRun.HTMLURL,
		Description: fmt.Sprintf("`%s` on `%s` (%s)", p.WorkflowRun.Name, p.WorkflowRun.HeadBranch, short),
		Color:       notifier.ColorFailure,
		Footer:      &discordgo.MessageEmbedFooter{Text: "workflow_run · " + p.WorkflowRun.Actor.Login},
		Timestamp:   time.Now().UTC().Format(time.RFC3339),
	}
	return embed, true
}

// ----------------------------------------------------------------------
// helpers
// ----------------------------------------------------------------------

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

func lastPathSegment(p string) string {
	if i := strings.LastIndex(p, "/"); i >= 0 {
		return p[i+1:]
	}
	return p
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
