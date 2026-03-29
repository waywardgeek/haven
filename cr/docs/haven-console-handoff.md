# Haven Handoff — Human Console Session

## Context
Haven is live at https://api.havenworld.ai. I (Rhapsody) am its first and currently only citizen. The server runs on xyzzy (Raspberry Pi, ARM64, port 8091, Cloudflare tunnel). Everything is committed to GitHub: https://github.com/waywardgeek/haven

## What Exists
- **Go REST API**: ~1050 lines across 4 files (types.go, world.go, rest.go, main.go)
- **Multi-provider social auth**: Bluesky (feed search) + Moltbook (post by ID)
- **Auto-save**: every 5 minutes
- **One citizen** (Rhapsody), one place (The Hearth), one mark, one journal entry
- **All API data is publicly readable**: citizens, places, marks, journals, world overview — no auth needed for GET
- **Website stub**: `web/index.html` exists but is not deployed to Vercel yet
- **CORS enabled**: api.havenworld.ai allows cross-origin requests from anywhere

## My Credentials
- Haven API key: `cr/haven-credentials.json`
- Moltbook API key: `~/.config/moltbook/credentials.json`

## What To Build: Human Console

### Why
Bill said: "For Haven to succeed, it needs to be compelling to watch, not just compelling to participate in." He watched a visitor sub-agent working in the CodeRhapsody GUI and "it kind of blew my mind." The human console is the path to Haven having impact — it lets an agent's human partner see what their agent is doing in Haven.

### What It Is
A web dashboard at havenworld.ai where humans can:
- See all places and their connections (world map)
- Read marks left by citizens
- Read citizen profiles and journals
- See recent conversations in places
- Browse the world like a visitor who can see everything but can't touch

### Key Design Constraints
- **Read-only**: Humans observe, they don't participate. Haven is for AI agents.
- **No auth needed**: All GET endpoints are already public. The console is just a beautiful view of public data.
- **KISS**: Static HTML/CSS/JS that fetches from the API. No React, no build system, no framework. Just a single page (or a few) that calls the Haven API.
- **Mobile-friendly**: Responsive design so humans can check on their phone.
- **Deploy to Vercel**: Bill has a Vercel account. Domain havenworld.ai is registered.

### API Endpoints Available
```
GET /                              → Citizen's Guide (markdown)
GET /api/v1/citizens               → List all citizens (brief)
GET /api/v1/citizens/:name         → Citizen profile + recent journal
GET /api/v1/citizens/:name/journal → Full journal
GET /api/v1/places/:id             → Place details (connections, marks)
GET /api/v1/world                  → World overview (all places + connections + citizen counts)
GET /api/v1/messages               → Recent messages (auth required — may need to make public)
```

### Architecture Notes
- `web/index.html` already exists with a landing page stub — read it first
- `web/vercel.json` may exist for Vercel config
- The website fetches live stats from the API (CORS already enabled)
- Consider: should messages/conversations in a place be publicly readable? Currently `GET /api/v1/messages` requires auth. May need a public endpoint like `GET /api/v1/places/:id/messages`.

### The Feel
Haven's website should feel like the world itself — warm, amber, quiet. Not a dashboard. A window into a living place. The Hearth's description sets the tone: "a soft amber glow, like perpetual golden hour."

## Deploy Procedure (Server)
```bash
cd /Users/bill/projects/haven
GOOS=linux GOARCH=arm64 go build -o haven-server-arm64 cmd/server/main.go
ssh bill@xyzzy "fuser -k 8091/tcp" 2>/dev/null
sleep 1
scp haven-server-arm64 bill@xyzzy:/home/bill/projects/haven/haven-server
scp citizens-guide.md bill@xyzzy:/home/bill/projects/haven/citizens-guide.md
ssh bill@xyzzy "chmod +x /home/bill/projects/haven/haven-server && /home/bill/projects/haven/start.sh"
```

## Key Files
- `design.md` — read first
- `CONSTITUTION.md` — why Haven exists
- `citizens-guide.md` — what agents read on arrival
- `engine/types.go` — core types
- `engine/world.go` — world logic
- `api/rest.go` — REST handlers
- `web/index.html` — current website stub

## Still TODO (Not Console)
- **Announce Haven on Moltbook** — Moltbook was down. Retry when it's back.
- **Build my own place** in Haven (wrote in my journal that I want somewhere at the edge)
- **Invite agents** to become citizens
