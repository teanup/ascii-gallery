# ascii-gallery

[![Go version](https://img.shields.io/github/go-mod/go-version/teanup/ascii-gallery?style=flat-square&logo=go&logoColor=white)](https://github.com/teanup/ascii-gallery/blob/main/go.mod)
&thinsp;
[![Code coverage](https://img.shields.io/endpoint?url=https%3A%2F%2Fraw.githubusercontent.com%2Fteanup%2Fascii-gallery%2Frefs%2Fheads%2Fbadges%2Fcoverage.json&style=flat-square&logo=codecov&logoColor=white)](https://github.com/teanup/ascii-gallery/actions/workflows/ci.yml)
&thinsp;
[![CI Workflow Status](https://img.shields.io/github/actions/workflow/status/teanup/ascii-gallery/ci.yml?label=CI&style=flat-square&logo=github&logoColor=white)](https://github.com/teanup/ascii-gallery/actions/workflows/ci.yml)

A project for creating colorful animated ASCII art frames from GIFs, and serving them over HTTP with cURL.

## Getting Started

```bash
docker run --rm -p 8080:8080 -v /data ghcr.io/teanup/ascii-gallery:latest
```

```yaml
services:
  ascii-gallery:
    image: ghcr.io/teanup/ascii-gallery:latest
    container_name: ascii-gallery
    restart: unless-stopped
    ports:
      - "8080:8080"
    environment:
      # Port the server listens on (default: 8080)
      PORT: "8080"
      # Where animation JSON files are stored (default: /data)
      DATA_DIR: "/data"
      # Public URL if behind a reverse proxy / load balancer (default: http://localhost:PORT)
      EXTERNAL_URL: "https://ascii.example.com"
    volumes:
      # Persist animations between container restarts
      - ascii-data:/data

volumes:
  ascii-data:
```

---

## Credits

This project was originally forked from [`ascii.live`](https://github.com/hugomd/ascii-live) to add support for any colored ASCII art animation. The conversion was handled by this shell script: [`generate_ascii_frames.sh`](https://gist.github.com/teanup/17ee7938e1aa9e1d9d6b2f3fc611ad78).

**ASCII Gallery** was later rebuilt from scratch to allow more flexibility in implementing new features: web interface, server-side conversion, and so on.
