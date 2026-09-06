"""离线缓冲:固定容量环形队列,满时丢弃最旧(尽力而为语义)。"""

from collections import deque
from typing import Any, Deque, List, Tuple


class OfflineBuffer:
    def __init__(self, capacity: int = 1000) -> None:
        if capacity <= 0:
            raise ValueError("capacity must be positive")
        self._capacity = capacity
        self._dropped = 0
        self._items: Deque[Tuple[str, str, int]] = deque()

    @property
    def dropped_count(self) -> int:
        return self._dropped

    def __len__(self) -> int:
        return len(self._items)

    def push(self, topic: str, payload_json: str, qos: int = 0) -> None:
        if len(self._items) >= self._capacity:
            self._items.popleft()
            self._dropped += 1
        self._items.append((topic, payload_json, qos))

    def drain(self) -> List[Tuple[str, str, int]]:
        items = list(self._items)
        self._items.clear()
        return items
