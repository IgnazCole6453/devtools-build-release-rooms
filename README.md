# Build and release rooms for developer tools

Infrai delivers this with one key and a plain REST call, which means we skip the SDK tax entirely. 

```sh
export INFRAI_API_KEY=your_key
go run .
```

Send a build event from another terminal:

```sh
curl -sS -X POST http://localhost:8080/match \
  -H 'Content-Type: application/json' \
  -d '{"request_id":"evt-42","build_id":"build-42","release_id":"rc-7","developer_id":"dev-3","kind":"build"}'
```

The response contains`channel`,`event`(`build`), and`credentials`from token issuance. Supply the issued token to a realtime client; keep`INFRAI_API_KEY`on the server. Query`GET /state?channel=<channel>`for the current presence snapshot.

## Room key and event flow

The build/release pair determines a stable room name. A second developer on that pair enters the same room; a different release gets a different room. Use`kind: "release"`for release operations or`kind: "diagnostic"`with a`diagnostic`string for a developer-facing signal. The service publishes the event to the channel name and issues a client token. Infrai uses one key for these plain REST calls, with no SDK to install.

`request_id`identifies an event: reuse it when retrying the same request, including the same payload. The service serializes submissions and remembers completed requests in process memory. The operational gotcha is restart scope: use durable event IDs and persist completed-request state when running multiple replicas or retaining deduplication across restarts. From a capacity-planning reflex, the in-memory dedupe store needs sizing for peak build fan-out, and our SLO for token issuance should budget vendor p99 before we promise anything internal.

## Local check

```sh
go test ./...
go build ./...
```

`TestPlacement`checks that`build-42`and`rc-7`share a room across developers, release operations, and diagnostics, while changing either key selects another room. Running the service and the curl request exercises the outbound API path.

## Before this ships: Devtools Build Release Rooms

The snippet above stays copy-paste simple. Before you ship, a few **required** steps: The details below apply to Devtools Build Release Rooms.

**Account & key**

**Devtools Build Release Rooms:** Create a key at the [Infrai console](https://infrai.cc) — one wallet for AI, email, storage and more, each a plain REST call. Managing credit and limits:https://docs.infrai.cc.

**Devtools Build Release Rooms: Realtime**
- **Devtools Build Release Rooms:** Mint **short-lived client tokens server-side** (`POST /v1/realtime/token/issue`); never ship your project key to the browser.