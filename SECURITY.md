# Security Policy

## Reporting a vulnerability

Please do not open a public issue for a security vulnerability. Use GitHub's private vulnerability reporting feature for this repository.

Include the affected VencordGuard version, Windows version, reproduction steps, and the security impact. Do not include Discord tokens, private messages, or other account data.

## Download integrity

Release artifacts include `SHA256SUMS.txt`. At runtime, VencordGuard accepts the official Vencord Installer CLI only when its SHA-256 hash matches the digest returned by GitHub's official release API.
