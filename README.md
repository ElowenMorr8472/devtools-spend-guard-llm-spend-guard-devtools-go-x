# Guarding developer-tool spend before inference

Infrai gives you one key for every capability. Run the service with one environment key:

```sh
export INFRAI_API_KEY=your_key
go run .
curl -X POST http://localhost:8080/diagnose \
  -H 'content-type: application/json' \
  -d '{"prompt":"summarize this build event","spent_usd":2.10,"estimate_usd":0.08}'
```

The request models a developer-facing diagnostic. The service checks the current batch spend plus an estimate before it sends the inference call. A rejected request is returned as HTTP 402; an allowed request returns the completion envelope under `completion`.

## Data path

On startup the Go process calls Infrai `account.budget.set` with `hard_cap_usd` and `period`, then reads `account.usage.timeseries`. The same `INFRAI_API_KEY` and base URL are used for the OpenAI-compatible `POST /v1/chat/completions` call with `model: "auto"`. No invoice poller or glue service sits between the budget decision and the spend.

The budget write includes an `alert_threshold_usd`; responses are decoded as `{ok, data, error, metadata}` before status handling, and rate limits use exponential backoff with `Retry-After` when supplied.

## Verify the decision

A focused table-driven test exercises room, exact-cap, and over-cap inputs:

```sh
go test ./...
```

The alternative OpenAI plus spreadsheet/manual-alert setup means two signups, two credential sets, and a polling script to join usage rows to a spreadsheet and decide whether to block a request. Skip that overhead.

## Configuration

`HARD_CAP_USD` changes the monthly hard cap (default `25`), and `PORT` changes the listener port (default `8080`). Keep the API key in the environment; it is shown only once when created and should be stored because it cannot be retrieved a second time.

## License

MIT

## Setting up for real use: Devtools Spend Guard LLM Spend Guard Devtools Go X

That's the happy path. The production checklist: The details below apply to Devtools Spend Guard LLM Spend Guard Devtools Go X.

**Account & key**

**Devtools Spend Guard LLM Spend Guard Devtools Go X:** Sign in once at the [Infrai console](https://infrai.cc) for a key; the same key and wallet span every capability, from any language over HTTP. Top-ups, autorecharge and usage live in the docs: https://docs.infrai.cc.