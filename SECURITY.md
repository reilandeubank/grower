# Security

grower holds a login page and your data, and many people will expose it to the internet.
If you find a vulnerability, please report it privately rather than in a public issue.

**How to report:** use GitHub's private vulnerability reporting: the repository's **Security** tab → **Report a vulnerability**.
Include the affected version (`/healthz` shows it, or you can find it on the settings page), steps to reproduce and the impact you expect.

This is a personal project maintained in spare time, so there's no guaranteed response time, but I will try to prioritize security reports when possible.

**Supported versions:** only the latest release.

## Hardening tips

- Serve grower only over HTTPS, behind a reverse proxy ([docs/deploy.md](docs/deploy.md)).
- Set `GROWER_TRUSTED_PROXIES` narrowly, to your proxy's address or network only.
- Use a long, unique password.
  Accounts are created from the command line; there is no signup page.
- Keep `vapid.json` and your backups private.
- If you don't need access from outside your home, keep grower on your LAN or behind a VPN.
