import logging
import time
from typing import Any, Optional
from urllib.parse import urlparse

import aiohttp

from .metrics import upstream_errors_total, upstream_request_duration_seconds, upstream_requests_total

logger = logging.getLogger(__name__)


def _service_name(base_url: str) -> str:
    """Extract a short service label from the base URL for metric labels."""
    host = urlparse(base_url).hostname or base_url
    # e.g. "user-profile-service" from "http://user-profile-service:8080"
    return host.split(":")[0]


class BaseClient:
    def __init__(self, base_url: str) -> None:
        self.base_url = base_url.rstrip("/")
        self._service = _service_name(base_url)
        self._session: Optional[aiohttp.ClientSession] = None

    async def start(self) -> None:
        self._session = aiohttp.ClientSession(
            connector=aiohttp.TCPConnector(limit=100),
        )
        logger.info("HTTP client started: %s", self.base_url)

    async def stop(self) -> None:
        if self._session and not self._session.closed:
            await self._session.close()
            logger.info("HTTP client stopped: %s", self.base_url)

    @property
    def session(self) -> aiohttp.ClientSession:
        if not self._session or self._session.closed:
            raise RuntimeError("Client session is not started or already closed")
        return self._session

    def _record(self, method: str, status: str, duration: float) -> None:
        upstream_requests_total.labels(self._service, method, status).inc()
        upstream_request_duration_seconds.labels(self._service, method).observe(duration)

    async def _get(self, path: str, params: Optional[dict] = None) -> Any:
        url = f"{self.base_url}{path}"
        logger.debug("GET %s params=%s", url, params)
        start = time.monotonic()
        try:
            async with self.session.get(url, params=params) as resp:
                resp.raise_for_status()
                data = await resp.json()
                self._record("GET", str(resp.status), time.monotonic() - start)
                return data
        except aiohttp.ClientError as exc:
            upstream_errors_total.labels(self._service).inc()
            self._record("GET", "error", time.monotonic() - start)
            raise exc

    async def _post(self, path: str, json: Optional[dict] = None) -> Any:
        url = f"{self.base_url}{path}"
        logger.debug("POST %s body=%s", url, json)
        start = time.monotonic()
        try:
            async with self.session.post(url, json=json) as resp:
                resp.raise_for_status()
                data = await resp.json()
                self._record("POST", str(resp.status), time.monotonic() - start)
                return data
        except aiohttp.ClientError as exc:
            upstream_errors_total.labels(self._service).inc()
            self._record("POST", "error", time.monotonic() - start)
            raise exc

    async def _patch(self, path: str, json: Optional[dict] = None) -> Any:
        url = f"{self.base_url}{path}"
        logger.debug("PATCH %s body=%s", url, json)
        start = time.monotonic()
        try:
            async with self.session.patch(url, json=json) as resp:
                resp.raise_for_status()
                data = await resp.json()
                self._record("PATCH", str(resp.status), time.monotonic() - start)
                return data
        except aiohttp.ClientError as exc:
            upstream_errors_total.labels(self._service).inc()
            self._record("PATCH", "error", time.monotonic() - start)
            raise exc

    async def _delete(self, path: str) -> None:
        url = f"{self.base_url}{path}"
        logger.debug("DELETE %s", url)
        start = time.monotonic()
        try:
            async with self.session.delete(url) as resp:
                if resp.status not in (200, 204):
                    resp.raise_for_status()
                self._record("DELETE", str(resp.status), time.monotonic() - start)
        except aiohttp.ClientError as exc:
            upstream_errors_total.labels(self._service).inc()
            self._record("DELETE", "error", time.monotonic() - start)
            raise exc
