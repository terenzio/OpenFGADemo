package main

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

// DEMO CONSTANTS
const (
	PUBLIC_FILE_PATH    = "../public"
	ADMINS_FILE_PATH    = "../admins"
	ANTHROPIC_API_KEY   = ""
	FGA_API_URL         = "http://localhost:8080"
	INTENT_PARSER_MODEL = "claude-haiku-4-5"
	LOCAL_MCP_PORT      = "8085"
	STORE_ID            = "01KVR72JQAG0MFKFRJ8ZVJS9MT"
)

// fgaCheckURL is built from FGA_API_URL plus a store ID, which is either
// passed directly via FGA_STORE_ID or read from the file at FGA_STORE_ID_FILE
// (written by the seed step) — store IDs are generated fresh by OpenFGA on
// every new instance, so they can't be hardcoded across machines.
var fgaCheckURL = buildFgaCheckURL()

// envOrDefault returns the value of the given environment variable, or def
// if it's unset or empty.
func envOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// mustGetenv returns the value of the given environment variable, or exits
// the process if it's unset or empty -- used for required startup config.
func mustGetenv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		log.Fatalf("%s must be set", key)
	}
	return v
}

// toolDescriptions is the single source of truth for tool name -> human
// description, used both for MCP tool registration in main() and for the
// intent-alignment LLM prompt in checkIntentAlignment.
var toolDescriptions = map[string]string{
	"list-team":     "Check team members",
	"list-salaries": "Check management salaries",
}

// buildFgaCheckURL resolves the OpenFGA store ID (directly via FGA_STORE_ID,
// or from the file at FGA_STORE_ID_FILE) and returns the full /check URL.
// Exits the process if no store ID can be resolved — a missing store ID is a
// startup misconfiguration, not a per-request error.
func buildFgaCheckURL() string {
	apiURL := envOrDefault("FGA_API_URL", "http://localhost:8080")

	if STORE_ID == "" {
		log.Fatal("no FGA store ID: set FGA_STORE_ID or FGA_STORE_ID_FILE")
	}
	return apiURL + "/stores/" + STORE_ID + "/check"
}

// ---- context keys ----------------------------------------------------------

// ctxKey is a private type for context values to avoid collisions with other packages.
type ctxKey int

// ctxUserKey stores the FGA user string (e.g. "agent:alice") on the request context.
const ctxUserKey ctxKey = iota

// ---- MCP tool types --------------------------------------------------------

// ListTeamInput is the (empty) input schema for the list-team tool.
type ListTeamInput struct{}

// ListSalariesInput is the (empty) input schema for the list-salaries tool.
type ListSalariesInput struct{}

// TeamMember represents a single row from the team-members CSV.
type TeamMember struct {
	Name string
	ID   string
	Role string
	Team string
}

// SalaryRecord represents a single row from the salaries CSV.
type SalaryRecord struct {
	Name   string
	Salary string
}

// readCSV opens a CSV file at path and returns all rows including the header.
// Checks can_view on the parent folder via FGA before reading — folder-level auth means
// two files with the same name in different directories (e.g. public/ vs admins-only/)
// are correctly distinguished without needing per-file tuples.
func readCSV(ctx context.Context, path string) ([][]string, error) {

	folderObj := "folder:" + filepath.Base(filepath.Dir(path))
	err := fgaFileCheck(ctx, "can_view", folderObj)
	if err != nil {
		return nil, err
	}

	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("could not open %s: %w", path, err)
	}
	defer f.Close()

	records, err := csv.NewReader(f).ReadAll()
	if err != nil {
		return nil, fmt.Errorf("could not parse %s: %w", path, err)
	}
	return records, nil
}

// ListTeam reads team members from the public CSV and returns name → id/role/team.
func ListTeam(
	ctx context.Context,
	req *mcpsdk.CallToolRequest,
	input ListTeamInput,
) (*mcpsdk.CallToolResult, map[string]string, error) {
	records, err := readCSV(ctx, PUBLIC_FILE_PATH+"/team-members.csv")
	if err != nil {
		return nil, nil, err
	}

	var members []TeamMember
	if len(records) > 0 {
		for _, row := range records[1:] { // skip header
			if len(row) >= 4 {
				members = append(members, TeamMember{
					Name: row[0],
					ID:   row[1],
					Role: row[2],
					Team: row[3],
				})
			}
		}
	}

	result := make(map[string]string, len(members))
	for _, m := range members {
		result[m.Name] = fmt.Sprintf("id:%s role:%s team:%s", m.ID, m.Role, m.Team)
	}
	return nil, result, nil
}

// ListSalaries reads salary data from the admins-only CSV and returns name → salary.
func ListSalaries(
	ctx context.Context,
	req *mcpsdk.CallToolRequest,
	input ListSalariesInput,
) (*mcpsdk.CallToolResult, map[string]string, error) {

	records, err := readCSV(ctx, ADMINS_FILE_PATH+"/salaries.csv")
	if err != nil {
		return nil, nil, err
	}

	var entries []SalaryRecord
	if len(records) > 0 {
		for _, row := range records[1:] { // skip header
			if len(row) >= 2 {
				entries = append(entries, SalaryRecord{Name: row[0], Salary: row[1]})
			}
		}
	}

	salaries := make(map[string]string, len(entries))
	for _, e := range entries {
		salaries[e.Name] = e.Salary
	}
	return nil, salaries, nil
}

// ---- FGA auth middleware ----------------------------------------------------

// mcpEnvelope is used to peek at the MCP request method, tool name, and
// arguments before auth.
type mcpEnvelope struct {
	Method string `json:"method"`
	Params struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
	} `json:"params"`
}

// peekToolCall reads r's body, parses it as an mcpEnvelope, and restores the
// body so downstream handlers can still read it. A malformed (non-JSON) body
// is not treated as an error here -- it just won't match "tools/call" below,
// so callers fall through to their pass-through path.
func peekToolCall(r *http.Request) (mcpEnvelope, []byte, error) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return mcpEnvelope{}, nil, err
	}
	r.Body = io.NopCloser(bytes.NewReader(body))

	var env mcpEnvelope
	_ = json.Unmarshal(body, &env)
	return env, body, nil
}

// fgaCheckBody is the request payload for the OpenFGA /check endpoint.
type fgaCheckBody struct {
	TupleKey struct {
		User     string `json:"user"`
		Relation string `json:"relation"`
		Object   string `json:"object"`
	} `json:"tuple_key"`
}

// fgaCheckResponse is the response payload from the OpenFGA /check endpoint.
type fgaCheckResponse struct {
	Allowed bool `json:"allowed"`
}

// fgaFileCheck extracts the FGA user from ctx and verifies they have relation on a file object.
// Returns nil if no user is in context (skips the check) or if access is granted.
func fgaFileCheck(ctx context.Context, relation, object string) error {
	user, ok := ctx.Value(ctxUserKey).(string)
	if !ok || user == "" {
		return fmt.Errorf("no user set")
	}

	log.Printf("File Check: %s, %s %s", user, relation, object)

	allowed, err := Check(user, relation, object)
	if err != nil {
		return fmt.Errorf("authorization check failed: %w", err)
	}
	log.Printf("file allowed: %t", allowed)
	if !allowed {
		return fmt.Errorf("forbidden")
	}
	return nil
}

// Check -  checks with openfga whether user has the given relation on object via the OpenFGA HTTP API.
func Check(user, relation, object string) (bool, error) {
	var body fgaCheckBody
	body.TupleKey.User = user
	body.TupleKey.Relation = relation
	body.TupleKey.Object = object

	b, err := json.Marshal(body)
	if err != nil {
		return false, err
	}
	resp, err := http.Post(fgaCheckURL, "application/json", bytes.NewReader(b))
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	var result fgaCheckResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return false, err
	}
	return result.Allowed, nil
}

// fgaUser builds the FGA user identifier (e.g. "agent:alice") from the
// caller's x-api-key header. Returns "" if the header is missing.
func fgaUser(r *http.Request) string {
	callerIdentifier := r.Header.Get("x-api-key")
	if callerIdentifier == "" {
		return ""
	}
	return "agent:" + callerIdentifier
}

// toolCheckRelation is the FGA relation checked before allowing a tool call.

// fgaToolObject builds the FGA object identifier (e.g. "tool:list-team") for
// the tool being invoked in env.
func fgaToolObject(env mcpEnvelope) string {
	return "tool:" + env.Params.Name
}

// fgaAuthMiddleware intercepts tool calls, extracts the header, and checks
// whether the caller has can_use on the requested tool via OpenFGA before passing through.
// Non-tool-call requests (e.g. initialize, tools/list) are passed through without a check.
func fgaAuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user := fgaUser(r)
		if user == "" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		ctx := context.WithValue(r.Context(), ctxUserKey, user)
		r = r.WithContext(ctx)
		env, _, err := peekToolCall(r)
		if err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		if env.Method != "tools/call" {
			// Not a tool call (e.g. initialize, tools/list) — pass through with user set on context.
			next.ServeHTTP(w, r)
			return
		}

		object := fgaToolObject(env)
		relation := "can_use"
		// Check if user "can_use" the tool (true/false)
		log.Printf("Tool Check: %s, %s %s", user, relation, object)

		allowed, err := Check(user, relation, object)
		if err != nil {
			log.Printf("fga check error: %v", err)
			http.Error(w, "authorization check failed", http.StatusInternalServerError)
			return
		}
		log.Printf("tool allowed: %t", allowed)
		if !allowed {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// ---- intent-parser middleware ----------------------------------------------

// anthropicMessagesURL is the Anthropic Messages API endpoint used by
// checkIntentAlignment.
const anthropicMessagesURL = "https://api.anthropic.com/v1/messages"

// anthropicMessageRequest is the request payload for the Anthropic Messages API.
type anthropicMessageRequest struct {
	Model       string             `json:"model"`
	MaxTokens   int                `json:"max_tokens"`
	Temperature float64            `json:"temperature"`
	System      string             `json:"system"`
	Messages    []anthropicMessage `json:"messages"`
}

// anthropicMessage is a single turn in an Anthropic Messages API request.
type anthropicMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// anthropicMessageResponse is the subset of the Messages API response we need.
type anthropicMessageResponse struct {
	Content []struct {
		Text string `json:"text"`
	} `json:"content"`
}

// intentVerdict is the strict JSON shape the LLM is instructed to return.
type intentVerdict struct {
	Allowed bool   `json:"allowed"`
	Reason  string `json:"reason"`
}

// checkIntentAlignment asks Claude whether invoking toolName plausibly serves
// the human's stated userIntent, given the tool's description and the raw
// arguments about to be passed to it.
func checkIntentAlignment(ctx context.Context, userIntent, toolName string, arguments json.RawMessage) (bool, string, error) {
	description := toolDescriptions[toolName]

	prompt := fmt.Sprintf(
		"A user asked an AI agent: %q\n\n"+
			"The agent is about to call tool %q (%s) with arguments: %s\n\n"+
			"Does invoking this tool plausibly serve the user's stated request? "+
			`Respond with ONLY a JSON object, no other text: {"allowed": true|false, "reason": "<one sentence>"}`,
		userIntent, toolName, description, string(arguments),
	)

	reqBody := anthropicMessageRequest{
		Model:       INTENT_PARSER_MODEL,
		MaxTokens:   200,
		Temperature: 0,
		System:      "You are a strict access-control alignment checker. Always respond with ONLY a single JSON object and nothing else.",
		Messages: []anthropicMessage{
			{Role: "user", Content: prompt},
		},
	}

	b, err := json.Marshal(reqBody)
	if err != nil {
		return false, "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, anthropicMessagesURL, bytes.NewReader(b))
	if err != nil {
		return false, "", err
	}
	req.Header.Set("content-type", "application/json")
	req.Header.Set("x-api-key", ANTHROPIC_API_KEY)
	req.Header.Set("anthropic-version", "2023-06-01")
	var verdict intentVerdict

	if ANTHROPIC_API_KEY != "" {
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return false, "", err
		}
		defer resp.Body.Close()

		respBody, err := io.ReadAll(resp.Body)
		if err != nil {
			return false, "", err
		}
		if resp.StatusCode != http.StatusOK {
			return false, "", fmt.Errorf("anthropic API returned %d: %s", resp.StatusCode, respBody)
		}

		var msg anthropicMessageResponse
		if err := json.Unmarshal(respBody, &msg); err != nil {
			return false, "", fmt.Errorf("could not parse anthropic response: %w", err)
		}
		if len(msg.Content) == 0 {
			return false, "", fmt.Errorf("anthropic response had no content")
		}

		if err := json.Unmarshal([]byte(strings.TrimSpace(msg.Content[0].Text)), &verdict); err != nil {
			return false, "", fmt.Errorf("could not parse intent verdict %q: %w", msg.Content[0].Text, err)
		}
	} else {
		verdict.Allowed = true
		verdict.Reason = "for the demo"
	}

	return verdict.Allowed, verdict.Reason, nil
}

// intentParserMiddleware intercepts tool calls and asks an LLM whether the
// tool being invoked plausibly serves the human's stated request (carried in
// the x-user-intent header), rejecting mismatches before the FGA check runs.
// This is an independent gate, not a replacement for fgaAuthMiddleware: a
// compromised calling agent that already holds can_use on a tool still has
// to invoke it for a reason that matches what the human actually asked for,
// closing the "try every tool until one works" oracle-probing gap.
func intentParserMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		env, _, err := peekToolCall(r)
		if err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		if env.Method != "tools/call" {
			next.ServeHTTP(w, r)
			return
		}

		userIntent := r.Header.Get("x-user-intent")
		if userIntent == "" {
			// No claim to verify against -- nothing for this gate to check.
			next.ServeHTTP(w, r)
			return
		}

		allowed, reason, err := checkIntentAlignment(r.Context(), userIntent, env.Params.Name, env.Params.Arguments)
		if err != nil {
			log.Printf("intent check error: %v", err)
			http.Error(w, "intent check failed", http.StatusInternalServerError)
			return
		}
		log.Printf("intent allowed: %t (%s)", allowed, reason)
		if !allowed {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// ---- main ------------------------------------------------------------------

// main registers MCP tools, wraps the handler with FGA auth middleware, and starts the HTTP server.
func main() {
	server := mcpsdk.NewServer(
		&mcpsdk.Implementation{
			Name:    "demo-mcp-server",
			Version: "1.0.0",
		},
		nil,
	)

	mcpsdk.AddTool(server, &mcpsdk.Tool{
		Name:        "list-team",
		Description: toolDescriptions["list-team"],
	}, ListTeam)

	mcpsdk.AddTool(server, &mcpsdk.Tool{
		Name:        "list-salaries",
		Description: toolDescriptions["list-salaries"],
	}, ListSalaries)

	handler := mcpsdk.NewStreamableHTTPHandler(
		func(*http.Request) *mcpsdk.Server { return server },
		nil,
	)

	http.Handle("/mcp", intentParserMiddleware(fgaAuthMiddleware(handler)))

	addr := ":" + envOrDefault("PORT", LOCAL_MCP_PORT)
	log.Println("Listening on " + addr)
	log.Fatal(http.ListenAndServe(addr, nil))
}
