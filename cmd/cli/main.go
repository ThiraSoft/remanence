// Command rem is the Rémanence command-line client.
//
// It creates and retrieves ephemeral burn-after-read messages against a
// running Rémanence server, with zero external dependencies.
package main

import (
	"bufio"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// version is the CLI build version, injected at build time via -ldflags.
var version = "dev"

// httpClient carries a timeout so the CLI never hangs on an unresponsive
// server or a network blackhole.
var httpClient = &http.Client{Timeout: 15 * time.Second}

const (
	defaultServer = "http://localhost:8080"
	maxLifeLimit  = 1440 // minutes (24h)
	defaultMaxLen = 2048 // server MAX_MESSAGE_LENGTH fallback
	gcmOverhead   = 28   // 12-byte IV + 16-byte GCM tag
)

// ANSI styling — disabled automatically when stdout is not a terminal.
var (
	cReset, cBold, cDim, cRed, cGreen, cCyan, cYellow string
)

func initColors() {
	if os.Getenv("NO_COLOR") != "" || !isTerminal(os.Stdout) {
		return
	}
	cReset = "\033[0m"
	cBold = "\033[1m"
	cDim = "\033[2m"
	cRed = "\033[31m"
	cGreen = "\033[32m"
	cCyan = "\033[36m"
	cYellow = "\033[33m"
}

func isTerminal(f *os.File) bool {
	fi, err := f.Stat()
	return err == nil && (fi.Mode()&os.ModeCharDevice) != 0
}

func main() {
	initColors()
	args := os.Args[1:]
	if len(args) == 0 {
		usage()
		os.Exit(1)
	}

	cmd := args[0]
	rest := args[1:]

	switch cmd {
	case "send", "s":
		cmdSend(rest)
	case "get", "g", "read":
		cmdGet(rest)
	case "version", "v":
		cmdVersion(rest)
	case "help", "-h", "--help":
		usage()
	default:
		fail("unknown command %q — run %srem help%s", cmd, cBold, cReset)
	}
}

// ---------------------------------------------------------------------------
// send
// ---------------------------------------------------------------------------

func cmdSend(args []string) {
	var (
		server  = serverFromEnv()
		message string
		hasMsg  bool
		ttl     = maxLifeLimit
		keep    bool // keep = do NOT burn after read
		asJSON  bool
		quiet   bool
	)

	pos := []string{}
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch a {
		case "-m", "--message":
			message, hasMsg = nextArg(args, &i, a), true
		case "-t", "--ttl":
			ttl = parseTTL(nextArg(args, &i, a))
		case "-s", "--server":
			server = normalizeServer(nextArg(args, &i, a))
		case "--keep":
			keep = true
		case "--json":
			asJSON = true
		case "-q", "--quiet":
			quiet = true
		case "-h", "--help":
			sendUsage()
			return
		default:
			if strings.HasPrefix(a, "-") {
				fail("send: unknown flag %q", a)
			}
			pos = append(pos, a)
		}
	}

	// Resolve message content: -m flag > positional args > stdin.
	if !hasMsg {
		if len(pos) > 0 {
			message = strings.Join(pos, " ")
		} else if !isTerminal(os.Stdin) {
			b, _ := io.ReadAll(os.Stdin)
			message = string(b)
		} else {
			message = prompt("Secret message: ")
		}
	}
	message = strings.TrimSpace(message)
	if message == "" {
		fail("send: message is empty")
	}

	// Ask the server whether end-to-end encryption is active. When it is,
	// encrypt locally with the exact same scheme as the web front-end, so
	// links are interchangeable between the CLI and the browser.
	encryptionOn, maxLen := fetchAbout(server)

	budget := maxLen
	if encryptionOn {
		budget = maxLen/4*3 - gcmOverhead // plaintext bytes that fit the envelope
	}
	if len(message) > budget {
		fail("send: message too large (%d bytes, max %d)", len(message), budget)
	}

	content, key := message, ""
	if encryptionOn {
		content, key = encrypt(message)
	}

	form := url.Values{}
	form.Set("content", content)
	form.Set("lifeLimit", strconv.Itoa(ttl))
	form.Set("isOneShot", strconv.FormatBool(!keep))

	resp, err := httpClient.PostForm(server+"/api/v1/messages", form)
	if err != nil {
		fail("send: cannot reach server at %s — %v", server, err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusCreated {
		fail("send: server rejected message (%s) — %s", resp.Status, serverError(body))
	}

	var out struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(body, &out); err != nil || out.ID == "" {
		fail("send: unexpected server response — %s", strings.TrimSpace(string(body)))
	}

	// The key travels only in the URL fragment — never sent to the server.
	shareURL := server + "/?message=" + out.ID
	if key != "" {
		shareURL += "#" + key
	}

	if asJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		enc.Encode(map[string]string{"id": out.ID, "url": shareURL})
		return
	}
	if quiet {
		fmt.Println(shareURL)
		return
	}

	copied := copyClipboard(shareURL)

	fmt.Printf("\n  %s%s✓ Message stored%s\n\n", cBold, cGreen, cReset)
	fmt.Printf("  %sLink%s   %s%s%s\n", cDim, cReset, cCyan, shareURL, cReset)
	fmt.Printf("  %sID%s     %s\n", cDim, cReset, out.ID)
	fmt.Printf("  %sExpires%s in %s%s\n", cDim, cReset, humanTTL(ttl), cReset)
	if keep {
		fmt.Printf("  %sMode%s   stays until expiry (multi-read)\n", cDim, cReset)
	} else {
		fmt.Printf("  %sMode%s   %sburns on first read%s\n", cDim, cReset, cYellow, cReset)
	}
	if encryptionOn {
		fmt.Printf("  %sCrypto%s end-to-end (AES-256-GCM) — key lives in the link only\n", cDim, cReset)
	} else {
		fmt.Printf("  %sCrypto%s %sdisabled on this server — stored as plaintext%s\n", cDim, cReset, cYellow, cReset)
	}
	if copied {
		fmt.Printf("\n  %s↳ copied to clipboard%s\n", cDim, cReset)
	}
	fmt.Println()
}

// ---------------------------------------------------------------------------
// get
// ---------------------------------------------------------------------------

func cmdGet(args []string) {
	var (
		server = serverFromEnv()
		asJSON bool
		raw    bool
		yes    bool
		target string
		key    string
	)

	for i := 0; i < len(args); i++ {
		a := args[i]
		switch a {
		case "-s", "--server":
			server = normalizeServer(nextArg(args, &i, a))
		case "-k", "--key":
			key = nextArg(args, &i, a)
		case "--json":
			asJSON = true
		case "-r", "--raw":
			raw = true
		case "-y", "--yes":
			yes = true
		case "-h", "--help":
			getUsage()
			return
		default:
			if strings.HasPrefix(a, "-") {
				fail("get: unknown flag %q", a)
			}
			target = a
		}
	}
	if target == "" {
		fail("get: missing message id or link")
	}

	id := extractID(target)
	// If a full URL was passed, prefer its host and embedded key.
	if u, err := url.Parse(target); err == nil {
		if u.Host != "" {
			server = u.Scheme + "://" + u.Host
		}
		if key == "" {
			key = u.Fragment
		}
	}

	// Reading may consume a burn-after-read message. Warn before doing so
	// when interactive; scripts (no TTY) and --yes skip the prompt.
	if !yes && isTerminal(os.Stdin) {
		confirm(fmt.Sprintf("%sThis reads the message and may destroy it permanently. Continue? [y/N] %s", cYellow, cReset))
	}

	resp, err := httpClient.Get(server + "/api/v1/messages/" + url.PathEscape(id))
	if err != nil {
		fail("get: cannot reach server at %s — %v", server, err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		fail("get: server error (%s) — %s", resp.Status, serverError(body))
	}

	var out struct {
		ID      string `json:"id"`
		Content string `json:"content"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		fail("get: unexpected server response — %s", strings.TrimSpace(string(body)))
	}

	plain := out.Content
	if key != "" {
		p, err := decrypt(out.Content, key)
		if err != nil {
			// Either the message is gone (server returns decoy content) or the
			// key is wrong — both are indistinguishable by design.
			fail("get: this message is gone, or the key is wrong")
		}
		plain = p
	} else if on, _ := fetchAbout(server); on {
		fail("get: this server encrypts messages — use a full link (…#<key>) or pass --key")
	}

	if asJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		enc.Encode(map[string]string{"id": out.ID, "content": plain})
		return
	}
	if raw {
		fmt.Println(plain)
		return
	}

	fmt.Printf("\n  %s%sMessage %s%s\n\n", cBold, cReset, cDim+out.ID, cReset)
	fmt.Printf("%s\n", plain)
	fmt.Printf("\n  %s⚠ If this was a burn-after-read message, it is now gone.%s\n\n", cDim, cReset)
}

// ---------------------------------------------------------------------------
// version
// ---------------------------------------------------------------------------

func cmdVersion(args []string) {
	server := serverFromEnv()
	for i := 0; i < len(args); i++ {
		if a := args[i]; a == "-s" || a == "--server" {
			server = normalizeServer(nextArg(args, &i, a))
		}
	}

	fmt.Printf("%srem%s %s — Rémanence CLI client\n", cBold, cReset, version)

	resp, err := httpClient.Get(server + "/api/v1/about")
	if err != nil {
		fmt.Printf("  server  %sunreachable at %s%s\n", cDim, server, cReset)
		return
	}
	defer resp.Body.Close()
	var about struct {
		Name, Version, BuildDate string
	}
	json.NewDecoder(resp.Body).Decode(&about)
	fmt.Printf("  server  %s %s (built %s) @ %s\n", about.Name, about.Version, about.BuildDate, server)
}

// ---------------------------------------------------------------------------
// encryption
//
// Messages are encrypted client-side with AES-256-GCM. The server only ever
// sees ciphertext; the key lives in the URL fragment (#...) and is never
// sent over the network. This scheme is identical to the web frontend's, so
// links are interchangeable between the CLI and the browser.
// ---------------------------------------------------------------------------

// encrypt seals plaintext and returns base64 ciphertext plus a base64url key.
func encrypt(plaintext string) (cipherText, key string) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		fail("encryption failed: %v", err)
	}
	block, _ := aes.NewCipher(raw)
	gcm, _ := cipher.NewGCM(block)
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		fail("encryption failed: %v", err)
	}
	// Seal prepends the nonce: payload = nonce || ciphertext || tag.
	payload := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(payload), base64.RawURLEncoding.EncodeToString(raw)
}

// decrypt reverses encrypt. It fails for tampered, expired or unrelated data.
func decrypt(cipherText, key string) (string, error) {
	// Accept the key with or without base64url padding.
	raw, err := base64.RawURLEncoding.DecodeString(strings.TrimRight(strings.TrimSpace(key), "="))
	if err != nil || len(raw) != 32 {
		return "", errors.New("invalid key")
	}
	payload, err := base64.StdEncoding.DecodeString(strings.TrimSpace(cipherText))
	if err != nil {
		return "", errors.New("not decryptable")
	}
	block, _ := aes.NewCipher(raw)
	gcm, _ := cipher.NewGCM(block)
	if len(payload) < gcm.NonceSize() {
		return "", errors.New("not decryptable")
	}
	nonce, ct := payload[:gcm.NonceSize()], payload[gcm.NonceSize():]
	plain, err := gcm.Open(nil, nonce, ct, nil)
	if err != nil {
		return "", errors.New("not decryptable")
	}
	return string(plain), nil
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

func serverFromEnv() string {
	if s := strings.TrimSpace(os.Getenv("REMANENCE_SERVER")); s != "" {
		return normalizeServer(s)
	}
	return defaultServer
}

// fetchAbout queries /api/v1/about for the server's encryption mode and
// message-size limit. On any failure it returns the secure defaults
// (encryption on), matching the web front-end's behaviour.
func fetchAbout(server string) (encryption bool, maxLen int) {
	encryption, maxLen = true, defaultMaxLen
	resp, err := httpClient.Get(server + "/api/v1/about")
	if err != nil {
		return
	}
	defer resp.Body.Close()
	var about struct {
		Encryption       *bool `json:"encryption"`
		MaxMessageLength int   `json:"maxMessageLength"`
	}
	if json.NewDecoder(resp.Body).Decode(&about) != nil {
		return
	}
	if about.Encryption != nil {
		encryption = *about.Encryption
	}
	if about.MaxMessageLength > 0 {
		maxLen = about.MaxMessageLength
	}
	return
}

func normalizeServer(s string) string {
	s = strings.TrimRight(strings.TrimSpace(s), "/")
	if !strings.HasPrefix(s, "http://") && !strings.HasPrefix(s, "https://") {
		s = "http://" + s
	}
	return s
}

// nextArg consumes the value following a flag, advancing the index.
func nextArg(args []string, i *int, flag string) string {
	if *i+1 >= len(args) {
		fail("flag %s needs a value", flag)
	}
	*i++
	return args[*i]
}

// parseTTL accepts "30", "30m", "2h" and clamps to 1..1440 minutes.
func parseTTL(s string) int {
	s = strings.TrimSpace(strings.ToLower(s))
	mult := 1
	switch {
	case strings.HasSuffix(s, "h"):
		mult, s = 60, strings.TrimSuffix(s, "h")
	case strings.HasSuffix(s, "m"):
		s = strings.TrimSuffix(s, "m")
	}
	n, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil {
		fail("invalid ttl %q — use minutes (90), or 30m / 2h", s)
	}
	n *= mult
	if n < 1 {
		n = 1
	}
	if n > maxLifeLimit {
		n = maxLifeLimit
	}
	return n
}

func humanTTL(min int) string {
	switch {
	case min%60 == 0 && min >= 60:
		return fmt.Sprintf("%dh", min/60)
	case min > 60:
		return fmt.Sprintf("%dh%dm", min/60, min%60)
	default:
		return fmt.Sprintf("%dm", min)
	}
}

// extractID pulls a message id out of a raw id or a share/API link.
func extractID(s string) string {
	s = strings.TrimSpace(s)
	if u, err := url.Parse(s); err == nil && u.Host != "" {
		if m := u.Query().Get("message"); m != "" {
			return m
		}
		// .../api/v1/messages/{id} or trailing path segment
		parts := strings.Split(strings.Trim(u.Path, "/"), "/")
		if len(parts) > 0 && parts[len(parts)-1] != "" {
			return parts[len(parts)-1]
		}
	}
	return s
}

// serverError extracts a human message from a JSON or plain error body.
func serverError(body []byte) string {
	var e struct {
		Error string `json:"error"`
	}
	if json.Unmarshal(body, &e) == nil && e.Error != "" {
		return e.Error
	}
	return strings.TrimSpace(string(body))
}

// prompt reads a single line from the terminal with the input hidden, so a
// typed secret is not left on screen for shoulder-surfers.
func prompt(label string) string {
	fmt.Fprint(os.Stderr, label)
	restore := hideInput()
	line, _ := bufio.NewReader(os.Stdin).ReadString('\n')
	restore()
	fmt.Fprintln(os.Stderr) // the hidden Enter leaves the cursor in place
	return line
}

// hideInput best-effort disables terminal echo via stty and returns a function
// that restores it. It is a no-op when stdin is not a terminal.
func hideInput() func() {
	if !isTerminal(os.Stdin) {
		return func() {}
	}
	if _, err := exec.LookPath("stty"); err != nil {
		return func() {}
	}
	off := exec.Command("stty", "-echo")
	off.Stdin = os.Stdin
	if off.Run() != nil {
		return func() {}
	}
	return func() {
		on := exec.Command("stty", "echo")
		on.Stdin = os.Stdin
		_ = on.Run()
	}
}

// confirm asks a yes/no question and exits cleanly unless the answer is yes.
func confirm(question string) {
	fmt.Fprint(os.Stderr, question)
	line, _ := bufio.NewReader(os.Stdin).ReadString('\n')
	switch strings.ToLower(strings.TrimSpace(line)) {
	case "y", "yes":
		return
	default:
		fmt.Fprintln(os.Stderr, "aborted.")
		os.Exit(0)
	}
}

// copyClipboard best-effort copies text using whatever tool is available.
func copyClipboard(text string) bool {
	candidates := [][]string{
		{"wl-copy"},
		{"xclip", "-selection", "clipboard"},
		{"xsel", "--clipboard", "--input"},
		{"pbcopy"},
	}
	for _, c := range candidates {
		if _, err := exec.LookPath(c[0]); err != nil {
			continue
		}
		cmd := exec.Command(c[0], c[1:]...)
		cmd.Stdin = strings.NewReader(text)
		if cmd.Run() == nil {
			return true
		}
	}
	return false
}

func fail(format string, a ...any) {
	fmt.Fprintf(os.Stderr, "%s%serror:%s "+format+"\n", append([]any{cRed, cBold, cReset}, a...)...)
	os.Exit(1)
}

// ---------------------------------------------------------------------------
// usage
// ---------------------------------------------------------------------------

func usage() {
	fmt.Printf(`%sRémanence CLI%s — ephemeral burn-after-read messages

%sUSAGE%s
  rem <command> [options]

%sCOMMANDS%s
  send     Create an ephemeral message and get a share link
  get      Retrieve a message by id or link
  version  Show client and server version
  help     Show this help

%sEXAMPLES%s
  rem send "deploy token: abc123"
  echo "$SECRET" | rem send --ttl 30m
  rem send -m "read me twice" --keep --ttl 2h
  rem get "https://host/?message=aBcDeFgHiJkLmNoP#<key>"
  rem get aBcDeFgHiJkLmNoP --key <key> --raw

%sENVIRONMENT%s
  REMANENCE_SERVER  Server base URL (default %s)
  NO_COLOR          Disable colored output

Run %srem <command> -h%s for command-specific options.
`, cBold, cReset, cBold, cReset, cBold, cReset, cBold, cReset, cBold, cReset, defaultServer, cBold, cReset)
}

func sendUsage() {
	fmt.Printf(`%srem send%s — create an ephemeral message

  rem send [message...]            message from arguments
  echo secret | rem send           message from stdin
  rem send                         interactive prompt

%sOPTIONS%s
  -m, --message <text>   Message content
  -t, --ttl <dur>        Lifetime: minutes, or 30m / 2h (default 24h, max 24h)
      --keep             Keep after read (default: burn on first read)
  -s, --server <url>     Server base URL
  -q, --quiet            Print only the share link
      --json             Output JSON

With no message argument and no pipe, the message is typed at a hidden prompt.
`, cBold, cReset, cBold, cReset)
}

func getUsage() {
	fmt.Printf(`%srem get%s — retrieve a message

  rem get <link>           full link, key read from the #… fragment
  rem get <id> --key <k>   id and key passed separately

%sOPTIONS%s
  -k, --key <key>    Decryption key (when not embedded in a link)
  -r, --raw          Print only the message content
  -y, --yes          Skip the "may destroy the message" confirmation
  -s, --server <url> Server base URL
      --json         Output JSON

The decryption key never reaches the server; without it a message
cannot be read. A burn-after-read message is consumed on first get —
when run interactively, rem asks for confirmation first.
`, cBold, cReset, cBold, cReset)
}
