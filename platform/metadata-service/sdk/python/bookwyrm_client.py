import requests


class BookwyrmClient:
    def __init__(self, base_url: str, api_key: str | None = None, timeout: int = 15):
        self.base_url = base_url.rstrip("/")
        self.timeout = timeout
        self.session = requests.Session()
        self.session.headers.update({"Accept": "application/json"})
        if api_key:
            self.session.headers.update({"X-API-Key": api_key})

    def search(self, query: str) -> dict:
        return self._request("GET", "/v1/search", params={"q": query})

    def get_work(self, work_id: str) -> dict:
        return self._request("GET", f"/v1/work/{work_id}")

    def get_quality_report(self, limit: int = 25) -> dict:
        return self._request("GET", "/v1/quality/report", params={"limit": limit})

    def repair_quality(self, dry_run: bool = True, limit: int = 25, remove_invalid_identifiers: bool = True) -> dict:
        payload = {
            "dry_run": dry_run,
            "limit": limit,
            "remove_invalid_identifiers": remove_invalid_identifiers,
        }
        return self._request("POST", "/v1/quality/repair", json=payload)

    def _request(self, method: str, path: str, **kwargs) -> dict:
        url = f"{self.base_url}{path}"
        response = self.session.request(method, url, timeout=self.timeout, **kwargs)
        if response.status_code >= 400:
            try:
                message = response.json().get("error", response.text)
            except ValueError:
                message = response.text
            raise RuntimeError(f"API error ({response.status_code}): {message}")
        return response.json()
