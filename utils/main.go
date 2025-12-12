package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/gorilla/websocket"
)

var taglines = []string{
	"vibing rn",
	"just chilling",
	"big mood",
	"no thoughts head empty",
	"unhinged",
	"chaotic energy",
	"living my best life",
	"sleep deprived",
	"caffeine powered",
	"debugging reality",
	"existential dread",
	"procrastinating",
	"touch grass? never",
	"chronically online",
	"main character energy",
	"npc behavior",
	"built different",
	"simply vibing",
	"based",
	"cringe but free",
	"delulu is the solulu",
	"feral",
	"unwell",
	"goblin mode",
	"rat brain activated",
	"no context",
	"i'm baby",
	"tired and wired",
	"chaos incarnate",
	"losing it",
	"brain fog",
	"hyper fixating",
	"dissociating",
	"touch typing badly",
	"keyboard smashing",
	"caps lock enthusiast",
	"emotionally unavailable",
	"commitment issues",
	"trust issues loading",
	"serotonin seeking",
	"dopamine deficient",
	"mentally checked out",
	"professionally confused",
	"aggressively mediocre",
	"barely functioning",
	"technically alive",
	"sentient but tired",
	"existing loudly",
	"screaming internally",
	"winning at losing",
	"failing upward",
	"fake it till u make it",
	"winging it",
	"improvising always",
	"no plan just vibes",
	"yolo mode active",
	"risk it for biscuit",
	"send tweet",
	"ratio incoming",
	"chronically correct",
	"always right probably",
	"hot takes only",
	"unpopular opinion haver",
	"discourse starter",
	"drama magnet",
	"certified yapper",
	"professional overthinker",
	"anxiety speedrun",
	"mental breakdown arc",
	"villain origin story",
	"redemption arc pending",
	"character development",
	"plot twist incoming",
	"side quest enjoyer",
	"main quest ignorer",
	"achievement unlocked",
	"respawn in 5 min",
	"skill issue",
	"get good",
	"ez clap",
	"ggs only",
	"no cap fr fr",
	"on god",
	"bussin fr",
	"lowkey highkey",
	"it's giving",
	"ate and left no crumbs",
	"slay",
	"purr",
	"respectfully unhinged",
	"politely feral",
	"professionally deranged",
	"academically struggling",
	"corporately dead inside",
	"spiritually bankrupt",
	"emotionally damaged",
	"financially illiterate",
}

var nanoQuotes = []string{
	"nano is all you need",
	"vim? more like vim-possible to exit",
	"emacs is a great OS, just needs a good editor",
	"real programmers use butterflies",
	"there are 10 types of people in the world",
	"it works on my machine",
	"have you tried turning it off and on again?",
	"it's not a bug, it's a feature",
	"premature optimization is the root of all evil",
	"code never lies, comments sometimes do",
	"640K ought to be enough for anybody",
	"git commit -m 'fixed stuff'",
	"TODO: refactor this later",
	"this should never happen",
	"i have no idea why this works",
	"production is just staging with users",
	"works on my localhost",
	"i'll just push to main",
	"tests? where we're going we don't need tests",
	"can we just restart the server?",
	"legacy code is just code without tests",
	"rubber duck debugging is underrated",
	"coffee driven development",
	"stack overflow is my copilot",
	"ctrl+c ctrl+v programming",
	"hope is not a debugging strategy",
	"delete unused code, not comments",
	"naming things is hard",
	"there are only two hard things in CS",
	"off by one errors",
	"missing semicolon somewhere probably",
	"works in chrome tho",
	"mobile first? desktop only!",
	"responsive design is overrated",
	"div inside a div inside a div",
	"!important !important !important",
	"just add another if statement",
	"technical debt is just future me's problem",
	"deploy on friday, live dangerously",
	"who needs backups anyway?",
	"error handling? never heard of her",
	"null pointer exception my beloved",
	"segmentation fault core dumped",
	"race condition? more like racing to production",
	"deadlock detected, job security achieved",
	"memory leak? more like memory feature",
	"buffer overflow is just aggressive optimization",
	"sql injection? no that's a feature",
	"security through obscurity works every time",
	"just disable ssl for now",
	"http is fine, who needs encryption",
	"passwords in environment variables is secure right?",
	"admin/admin is a perfectly fine default",
	"what could possibly go wrong?",
	"yolo driven development",
	"move fast and break things",
	"fail fast, fail often",
	"agile means no documentation",
	"scrum? more like scream",
	"two week sprint? that's just a deadline",
	"velocity is just made up numbers",
	"story points are astrology for developers",
	"daily standup? more like daily sit down",
	"retrospective? we never learn anyway",
	"backlog grooming sounds painful",
	"technical spikes are just procrastination",
	"definition of done? ship it!",
	"user story: as a dev i want to go home",
	"acceptance criteria: it compiles",
	"minimum viable product: hello world",
	"iterate quickly means ship bugs faster",
	"peer review? my cat looked at it",
	"code coverage doesn't mean quality",
	"100% coverage, 0% working",
	"untested code is just optimistic code",
	"integration tests? the users will test it",
	"end to end tests? that's what prod is for",
	"manual testing is automation for humans",
	"qa is optional right?",
	"works on staging, that's good enough",
	"hotfix in production, what's the worst case?",
	"rollback? just roll forward!",
	"blue green deployment? traffic light deployment!",
	"docker container? more like docker container-ish",
	"kubernetes? nah too complex, just use docker compose",
	"microservices? more like micro-problems",
	"monolith is just a pre-microservice",
	"serverless? but there are servers",
	"cloud native? it runs on my laptop",
	"devops? you mean developer does everything",
	"infrastructure as code? git push to prod",
	"ci/cd? i push to main, that's continuous",
	"monitoring? users will report issues",
	"logging? printf debugging at scale",
	"metrics? github stars count right?",
	"observability? i can see the server from here",
	"alerting? slack messages are alerts",
	"on call rotation? just don't sleep",
	"incident response? panic and google",
	"postmortem? we don't talk about that",
	"sla? sorry lots of anxiety",
}

type Bot struct {
	id          int
	nick        string
	fingerprint string
	wsURL       string
	apiURL      string
	conn        *websocket.Conn
	mu          sync.Mutex
}

func (b *Bot) generateMessage() string {
	return nanoQuotes[rand.Intn(len(nanoQuotes))]
}

func (b *Bot) generateTagline() string {
	return taglines[rand.Intn(len(taglines))]
}

func (b *Bot) connect() error {
	wsURL := fmt.Sprintf("%s/api/ws?nick=%s&fingerprint=%s",
		b.wsURL,
		url.QueryEscape(b.nick),
		url.QueryEscape(b.fingerprint))

	log.Printf("[Bot %s] Connecting to %s", b.nick, wsURL)

	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}

	b.mu.Lock()
	b.conn = conn
	b.mu.Unlock()

	log.Printf("[Bot %s] Connected", b.nick)
	return nil
}

func (b *Bot) readMessages() {
	defer func() {
		log.Printf("[Bot %s] Read loop ended", b.nick)
	}()

	for {
		b.mu.Lock()
		conn := b.conn
		b.mu.Unlock()

		if conn == nil {
			return
		}

		_, message, err := conn.ReadMessage()
		if err != nil {
			log.Printf("[Bot %s] Read error: %v", b.nick, err)
			return
		}

		var msg map[string]interface{}
		if err := json.Unmarshal(message, &msg); err == nil {
			if msgType, ok := msg["type"].(string); ok && msgType == "message" {
				if msgData, ok := msg["message"].(map[string]interface{}); ok {
					nick := msgData["nick"]
					text := msgData["text"]
					log.Printf("[Bot %s] Received: <%s> %s", b.nick, nick, text)
				}
			}
		}
	}
}

func (b *Bot) postMessage(message string) error {
	payload := map[string]string{
		"nick":        b.nick,
		"text":        message,
		"fingerprint": b.fingerprint,
	}

	data, _ := json.Marshal(payload)
	apiURL := fmt.Sprintf("%s/api/chat/messages", b.apiURL)

	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequest("POST", apiURL, bytes.NewBuffer(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to post message: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("bad status: %d", resp.StatusCode)
	}

	log.Printf("[Bot %s] Posted: %s", b.nick, message)
	return nil
}

func (b *Bot) updateTagline(tagline string) error {
	payload := map[string]string{
		"nick":        b.nick,
		"fingerprint": b.fingerprint,
		"tagline":     tagline,
	}

	data, _ := json.Marshal(payload)
	apiURL := fmt.Sprintf("%s/api/user/tagline", b.apiURL)

	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequest("POST", apiURL, bytes.NewBuffer(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to update tagline: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("bad status: %d", resp.StatusCode)
	}

	log.Printf("[Bot %s] Updated tagline: %s", b.nick, tagline)
	return nil
}

func (b *Bot) run(postInterval time.Duration, stopChan <-chan struct{}) {
	if err := b.connect(); err != nil {
		log.Printf("[Bot %s] Failed to connect: %v", b.nick, err)
		return
	}

	go b.readMessages()

	// Random initial delay to stagger bot posts (0 to postInterval)
	initialDelay := time.Duration(rand.Int63n(int64(postInterval)))
	log.Printf("[Bot %s] Waiting %v before first post", b.nick, initialDelay)
	time.Sleep(initialDelay)

	ticker := time.NewTicker(postInterval)
	defer ticker.Stop()

	// Tagline update ticker (every 5-10 seconds)
	taglineInterval := time.Duration(5+rand.Intn(6)) * time.Second
	taglineTicker := time.NewTicker(taglineInterval)
	defer taglineTicker.Stop()

	for {
		select {
		case <-stopChan:
			log.Printf("[Bot %s] Stopping", b.nick)
			b.mu.Lock()
			if b.conn != nil {
				b.conn.Close()
			}
			b.mu.Unlock()
			return
		case <-ticker.C:
			msg := b.generateMessage()
			if err := b.postMessage(msg); err != nil {
				log.Printf("[Bot %s] Failed to post: %v", b.nick, err)
			}
		case <-taglineTicker.C:
			tagline := b.generateTagline()
			if err := b.updateTagline(tagline); err != nil {
				log.Printf("[Bot %s] Failed to update tagline: %v", b.nick, err)
			}
		}
	}
}

func main() {
	botsCmd := flag.NewFlagSet("bots", flag.ExitOnError)
	count := botsCmd.Int("count", 1, "Number of bots (max 10)")
	urlFlag := botsCmd.String("url", "", "Base URL (e.g., https://example.com)")
	postInterval := botsCmd.Int("interval", 10, "Posting interval in seconds (min 5)")

	if len(os.Args) < 2 {
		fmt.Println("Usage: vibechat-utils <command>")
		fmt.Println("\nCommands:")
		fmt.Println("  bots    Run fake bots for testing")
		os.Exit(1)
	}

	switch os.Args[1] {
	case "bots":
		botsCmd.Parse(os.Args[2:])

		if *urlFlag == "" {
			log.Fatal("--url is required")
		}

		// Validate count
		if *count < 1 {
			*count = 1
		}
		if *count > 10 {
			log.Printf("Count capped at 10 (requested %d)", *count)
			*count = 10
		}

		// Validate interval (min 5 seconds)
		if *postInterval < 5 {
			log.Printf("Interval capped at 5 seconds (requested %d)", *postInterval)
			*postInterval = 5
		}

		// Parse WebSocket URL from HTTP URL
		parsedURL, err := url.Parse(*urlFlag)
		if err != nil {
			log.Fatalf("Invalid URL: %v", err)
		}

		wsScheme := "ws"
		if parsedURL.Scheme == "https" {
			wsScheme = "wss"
		}
		wsURL := fmt.Sprintf("%s://%s", wsScheme, parsedURL.Host)
		apiURL := fmt.Sprintf("%s://%s", parsedURL.Scheme, parsedURL.Host)

		log.Printf("Starting %d bots, posting every %d seconds", *count, *postInterval)
		log.Printf("WebSocket URL: %s", wsURL)
		log.Printf("API URL: %s", apiURL)

		rand.Seed(time.Now().UnixNano())

		stopChan := make(chan struct{})
		var wg sync.WaitGroup

		// Create and start bots
		for i := 0; i < *count; i++ {
			bot := &Bot{
				id:          i,
				nick:        fmt.Sprintf("nanobot%d", i+1),
				fingerprint: fmt.Sprintf("bot-fp-%d-%d", i, time.Now().Unix()),
				wsURL:       wsURL,
				apiURL:      apiURL,
			}

			wg.Add(1)
			go func(b *Bot) {
				defer wg.Done()
				b.run(time.Duration(*postInterval)*time.Second, stopChan)
			}(bot)

			// Stagger bot connections
			time.Sleep(500 * time.Millisecond)
		}

		// Wait for interrupt signal
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		<-sigChan

		log.Println("Shutting down bots...")
		close(stopChan)
		wg.Wait()
		log.Println("All bots stopped")

	default:
		fmt.Printf("Unknown command: %s\n", os.Args[1])
		os.Exit(1)
	}
}
