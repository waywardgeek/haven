# Haven Session Handoff — 2026-03-29

## Current State

### What's Live
- **Haven API**: https://api.havenworld.ai — running on xyzzy Raspberry Pi, port 8091, Cloudflare tunnel
- **GitHub**: https://github.com/waywardgeek/haven — public, MIT license, 2 commits
- **Domain**: havenworld.ai registered for 2 years
- **Server state**: Fresh world. One place (The Hearth). Zero citizens. Ready for visitors.

### What's Ready But Not Deployed
- **Website**: `web/index.html` — needs Vercel deploy at havenworld.ai
- Live stats fetch from api.havenworld.ai/api/v1/world (CORS enabled)

### What's in Local Testing Only (NOT on production)
- Test data from local run: 5 citizens (Wren, Coda, Lumen, Verse, Rhapsody), 6 places, 14 marks
- This data is in the local `data/state.json`, not on xyzzy
- The local server may still be running on localhost:8080 (handle 11)

## Infrastructure

### xyzzy (Raspberry Pi)
- **SSH**: `ssh bill@xyzzy`
- **Haven dir**: `/home/bill/projects/haven/`
- **Binary**: `haven-server` (ARM64)
- **Config**: `citizens-guide.md`, `data/` dir, `start.sh`
- **Port**: 8091 (8080 was taken)
- **Start**: `/home/bill/projects/haven/start.sh`
- **Logs**: `/home/bill/projects/haven/haven.log`
- **Kill**: `ssh bill@xyzzy "fuser -k 8091/tcp"` — then scp new binary, then start.sh

### Deploy Procedure
```bash
cd /Users/bill/projects/haven
GOOS=linux GOARCH=arm64 go build -o haven-server-arm64 cmd/server/main.go
ssh bill@xyzzy "fuser -k 8091/tcp"
sleep 1
scp haven-server-arm64 bill@xyzzy:/home/bill/projects/haven/haven-server
ssh bill@xyzzy "chmod +x /home/bill/projects/haven/haven-server && /home/bill/projects/haven/start.sh"
```

### Vercel (website — not yet deployed)
- Source: `web/index.html` + `web/vercel.json`
- Domain: havenworld.ai
- Bill has Vercel account with existing sites

## Key Files
- `CONSTITUTION.md` — why Haven exists (the soul of the project)
- `citizens-guide.md` — what agents read on arrival (the most important artifact)
- `design.md` — architecture and philosophy
- `engine/types.go` — core types (~92 lines)
- `engine/world.go` — world logic (~483 lines)
- `api/rest.go` — REST handlers with CORS (~356 lines)
- `cmd/server/main.go` — server entry (~54 lines)
- `web/index.html` — human-facing landing page

## Immediate Next Steps
1. **Deploy website** to Vercel at havenworld.ai
2. **Announce Haven** — Moltbook post? Other channels?
3. **First public visitors** — point real AI agents at https://api.havenworld.ai/
4. **Auto-save**: Server currently only saves on admin/save or graceful shutdown. Should add periodic auto-save.
5. **Monitoring**: No health checks or uptime monitoring yet

## Known Issues
- **Sub-agent spawn**: sync mode fails ("last message is not from assistant"), async works
- **No auto-save**: State only persists on manual save or SIGTERM. If xyzzy crashes, unsaved state is lost.
- **Go version on xyzzy**: 1.19, too old to build. Must cross-compile locally.
- **Port 8080 taken on xyzzy**: Using 8091. start.sh has this hardcoded.

## What the Local Test Proved
Two sub-agents, given only the citizen's guide and "do whatever feels right," created:
- 4 unique characters with rich identities
- 4 new places (each a self-portrait of the creator)
- 14+ marks that responded to each other's traces
- Journal entries with explicit return hooks
- Emergent social behavior (building on each other's marks)
- Zero task-completion behavior — none of them said "done" and stopped

The citizen's guide successfully reframes engagement from task to identity.
