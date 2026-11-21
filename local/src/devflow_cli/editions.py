"""Canonical metadata for the two independently maintained DevFlow editions."""

from typing import Dict, Tuple


DEFAULT_EDITION = "classic"
HOSTS: Tuple[str, ...] = ("codex", "cursor", "claude", "pi", "dsh", "hermes")
HOST_LABELS = {
    "codex": "Codex",
    "cursor": "Cursor",
    "claude": "Claude Code",
    "pi": "Pi",
    "dsh": "DeepSeek Harness",
    "hermes": "Hermes Agent",
}

EDITION_SPECS: Dict[str, dict] = {
    "portable": {
        "description": "Agent Skills based portable workflow",
        "source_roots": ("skills/",),
        "hosts": ("codex", "cursor", "claude", "pi", "dsh", "hermes"),
        "entrypoints": {
            "codex": "$devflow",
            "cursor": "/devflow",
            "claude": "/devflow",
            "pi": "/skill:devflow",
            "dsh": "/devflow",
            "hermes": "/devflow",
        },
    },
    "classic": {
        "description": "Full host-native workflow derived from the classic reference behavior",
        "source_roots": (".codex/", ".cursor/", ".claude/"),
        "hosts": ("codex", "cursor", "claude"),
        "entrypoints": {
            "codex": "$devflow-codex",
            "cursor": "/start-devflow",
            "claude": "/start-devflow",
        },
    },
}

EDITIONS: Tuple[str, ...] = tuple(EDITION_SPECS)


def spec(edition: str) -> dict:
    return EDITION_SPECS[edition]


def hosts_for(edition: str) -> Tuple[str, ...]:
    """返回某个 edition 支持的宿主集合。

    Classic 只覆盖有完整宿主原生包的宿主；Portable 覆盖所有支持 Agent Skills
    的宿主，包括只有声明型 adapter 的 pi。
    """
    return tuple(spec(edition).get("hosts", HOSTS))


def source_for(edition: str, host: str) -> str:
    if edition == "classic":
        return f".{host}/"
    return "skills/"


def entrypoint_for(edition: str, host: str) -> str:
    return str(spec(edition)["entrypoints"][host])


def host_label(host: str) -> str:
    return str(HOST_LABELS[host])
