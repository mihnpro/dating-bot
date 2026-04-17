import logging
from typing import Any, Optional

import aiohttp

logger = logging.getLogger(__name__)


class BaseClient:
    def __init__(self, base_url: str) -> None:
        self.base_url = base_url.rstrip("/")
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

    async def _get(self, path: str, params: Optional[dict] = None) -> Any:
        url = f"{self.base_url}{path}"
        logger.debug("GET %s params=%s", url, params)
        async with self.session.get(url, params=params) as resp:
            resp.raise_for_status()
            return await resp.json()

    async def _post(self, path: str, json: Optional[dict] = None) -> Any:
        url = f"{self.base_url}{path}"
        logger.debug("POST %s body=%s", url, json)
        async with self.session.post(url, json=json) as resp:
            resp.raise_for_status()
            return await resp.json()

    async def _patch(self, path: str, json: Optional[dict] = None) -> Any:
        url = f"{self.base_url}{path}"
        logger.debug("PATCH %s body=%s", url, json)
        async with self.session.patch(url, json=json) as resp:
            resp.raise_for_status()
            return await resp.json()

    async def _delete(self, path: str) -> None:
        url = f"{self.base_url}{path}"
        logger.debug("DELETE %s", url)
        async with self.session.delete(url) as resp:
            if resp.status not in (200, 204):
                resp.raise_for_status()
