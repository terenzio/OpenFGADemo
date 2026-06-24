## 0. PREREQUISITES
- Go installed
- An OpenFGA instance running locally (defaults to `http://localhost:8080`)
- A store created in that instance, with its ID set as `STORE_ID` in `main.go`

## 1. RUNNING YOUR LOCAL MCP SERVER
`cd ibac/demo`

`go mod tidy`

in main.go set constants to match your environment:

    - STORE_ID: required, your OpenFGA store ID — the server exits on startup if this is empty
    - FGA_API_URL: optional, the url to your local OpenFGA
    - ANTHROPIC_API_KEY: optional, your api key, blank for demo will return a mock response
    - LOCAL_MCP_PORT: optional, change if you 8085 is occupied


`go run main.go`

check that your application is listening on the port set for LOCAL_MCP_PORT

`2026/0X/2X 01:03:51 Listening on :8085`


## 2. REGISTER THE MCP SERVER WITH YOUR AGENT (claude example)

Use the `claude mcp add` CLI rather than hand-editing JSON under `projects.<path>.mcpServers` — that entry is keyed on an exact path string, so running the command from a subdirectory (e.g. `MCP/demo-mcp`) or a path with a trailing slash silently creates a separate, invisible entry that won't show up for your actual project directory.

1. Register the server at **user scope**, so it's available no matter which directory you're in. Make sure the port matches your `LOCAL_MCP_PORT`.
```
claude mcp add --transport http org-mcp http://localhost:8085/mcp \
  --header "x-api-key: ceo-bot-key" \
  -s user
```

2. Verify it was added:
```
claude mcp list
```
You should see `org-mcp` listed, regardless of your current working directory.

 - IMPORTANT! The `x-api-key` value we will change throughout the demo as we simulate different users to test their permission level. To rotate it, remove and re-add:
```
claude mcp remove org-mcp -s user
claude mcp add --transport http org-mcp http://localhost:8085/mcp --header "x-api-key: <new-key>" -s user
```

Alternative (manual edit): open `~/.claude.json` (`code ~/.claude.json`) and add the entry under the **top-level** `mcpServers` key — not nested under `projects.<path>.mcpServers`:
```
{
  "mcpServers": {
    "org-mcp": {
      "type": "http",
      "url": "http://localhost:8085/mcp",
      "headers": {
        "x-api-key": "ceo-bot-key"
      }
    }
  },
  ...
}
```

## 3. VERIFY THE CONNECTION (via terminal)
1. start your agent
```
claude
```
2. use the /mcp command to list available mcps, if your mcp is up and running, you should see something like:
   `org-mcp · ✔ connected · 2 tools` 

   - if you need to debug the connction you can try to reconnect manually and see if any errors pop up.

## 4. TEST YOUR MCP SERVER 

1. talk to your agent and say something like, 'hey can you list my team?' or 'show me salaries'. 
If it's working your agent will tell you that you dont have access.

## 5. Initial tuple setup

First, write the authorization model from `auth-model/tools-model.fga` to your store (e.g. via the `fga` CLI: `fga model write --store-id <your store id> auth-model/tools-model.fga`, or the `/stores/{store_id}/authorization-models` API).

Note: the model has a couple of `# 1.` / `# 2.` markers (e.g. for a `tool` and `agent` type) that get filled in live during the demo — don't worry if those aren't defined yet.

Then add tuples to your OpenFGA (using the /write API) to grant your test identity `can_use` on the tools you want it to access, e.g.:

```
curl --location 'http://localhost:8080/stores/[your store id here]/write' \
--header 'Content-Type: application/json' \
--data '{
    "writes": {
        "tuple_keys": [
            {
                "user": "agent:<your-test-identity>",
                "relation": "user",
                "object": "tool:list-team"
            }
        ]
    }
}'
```

Now you can follow along with the demo slides to create the required tuples and grant access to your agent!
