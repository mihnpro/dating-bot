"""Prometheus metrics for the Gateway Service.

All metrics are module-level singletons registered in the default registry.
The /metrics HTTP endpoint is served by a lightweight aiohttp server started
in main.py alongside the Aiogram polling loop.
"""

from prometheus_client import Counter, Gauge, Histogram

# ── Telegram bot metrics ───────────────────────────────────────────────────────

telegram_updates_total = Counter(
    "gateway_telegram_updates_total",
    "Total number of Telegram updates received.",
    ["update_type"],  # message | callback_query | inline_query | …
)

commands_total = Counter(
    "gateway_commands_total",
    "Total number of bot commands handled.",
    ["command"],  # start | createprofile | profile | browse | matches | …
)

callback_actions_total = Counter(
    "gateway_callback_actions_total",
    "Total number of inline keyboard callback actions.",
    ["action"],  # like | pass | view_match | who_liked_me | …
)

# ── Upstream service metrics ───────────────────────────────────────────────────

upstream_requests_total = Counter(
    "gateway_upstream_requests_total",
    "Total number of HTTP requests sent to upstream Go services.",
    ["service", "method", "status"],  # service=user-profile, method=GET, status=200/error
)

upstream_request_duration_seconds = Histogram(
    "gateway_upstream_request_duration_seconds",
    "Duration of HTTP requests to upstream Go services.",
    ["service", "method"],
    buckets=[0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1.0, 2.5, 5.0],
)

upstream_errors_total = Counter(
    "gateway_upstream_errors_total",
    "Total number of failed upstream requests (network errors, non-2xx).",
    ["service"],
)

# ── Bot health metrics ─────────────────────────────────────────────────────────

active_fsm_sessions = Gauge(
    "gateway_active_fsm_sessions",
    "Current number of active FSM (onboarding / edit profile) sessions.",
)
