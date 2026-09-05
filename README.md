# ascii-gallery

[![Latest Container Image](https://img.shields.io/github/v/tag/teanup/ascii-gallery?style=flat-square&logo=docker&logoColor=white&label=ascii-gallery)](https://github.com/teanup/ascii-gallery/pkgs/container/ascii-gallery)
&thinsp;
[![CI Workflow Status](https://img.shields.io/github/actions/workflow/status/teanup/ascii-gallery/ci.yml?style=flat-square&logo=github&logoColor=white&label=CI)](https://github.com/teanup/ascii-gallery/actions/workflows/ci.yml)
&thinsp;
[![Code Coverage](https://img.shields.io/endpoint?url=https%3A%2F%2Fraw.githubusercontent.com%2Fteanup%2Fascii-gallery%2Frefs%2Fheads%2Fbadges%2Fcoverage.json&style=flat-square&logo=codecov&logoColor=white)](https://github.com/teanup/ascii-gallery/actions/workflows/ci.yml)
&thinsp;
[![Go Version](https://img.shields.io/github/go-mod/go-version/teanup/ascii-gallery?style=flat-square&logo=go&logoColor=white)](https://github.com/teanup/ascii-gallery/blob/main/go.mod)

A web server that converts GIFs into animated, colored ASCII art streamable directly to your terminal.

**:clapper: Live Demo** &ndash; Try it out at [ascii.lag.tf](https://ascii.lag.tf) and in your terminal:

```bash
curl -L ascii.lag.tf/anim/parrot
```

### Features

- :tv: **Smart streaming**: Detects your client automatically to serve an ANSI stream for terminals.
- :space_invader: **Automatic conversion**: Upload any GIF &ndash; the server handles resizing, aspect-ratio correction, and frame optimization instantly.
- :recycle: **Full lifecycle management**: Create, update, list, and delete animations entirely via curl or the web UI.
- :art: **High-fidelity rendering**: Uses pixel averaging to preserve color depth and detail without blocky artifacts.

## Getting Started

Run the latest container image:

```bash
docker run --rm -p 8080:8080 -v ascii-data:/data ghcr.io/teanup/ascii-gallery:latest
```

Or use Docker Compose for a persistent setup:

```yaml
services:
  ascii-gallery:
    image: ghcr.io/teanup/ascii-gallery:latest
    container_name: ascii-gallery
    restart: unless-stopped
    ports:
      - "8080:8080"
    environment:
      PORT: "8080" # Listening port
      DATA_DIR: "/data" # Directory for storing animation JSON files
      EXTERNAL_URL: "http://localhost:8080" # Public URL if behind a reverse proxy
    volumes:
      - ascii-data:/data

volumes:
  ascii-data:
```

To run from source without Docker:

```bash
DATA_DIR=/tmp/ascii-data go run main.go
```

## Usage

### Play an animation

List available animations:

```bash
curl localhost:8080/anim
```

Stream a specific animation using its ID:

```bash
curl localhost:8080/anim/{id}
```

### Upload & Manage

Upload a GIF:

```bash
curl -F "file=@animation.gif" localhost:8080/anim
```

Upload with custom parameters (ID, name, width):

```bash
curl -F "file=@animation.gif" -F "id=myid" -F "name=My Animation" -F "width=120" localhost:8080/anim
```

Replace an existing animation:

```bash
curl -X PUT -F "file=@new.gif" localhost:8080/anim/{id}
```

Delete an animation:

```bash
curl -X DELETE localhost:8080/anim/{id}
```

**Upload Options:**

- `width` / `height`: Output size in characters (aspect ratio preserved).
- `max_frames`: Cap the number of frames (default/maximum: 100).
- `min_delay`: Minimum time between frames in ms (default/minimum: 50).

## API

| Method   | Path         | Description                                  |
| -------- | ------------ | -------------------------------------------- |
| `GET`    | `/`          | Text help (CLI) or Web UI (Browser)          |
| `GET`    | `/anim`      | List all animations (JSON)                   |
| `GET`    | `/anim/{id}` | Stream animation (CLI) or metadata (Browser) |
| `POST`   | `/anim`      | Upload a new GIF                             |
| `PUT`    | `/anim/{id}` | Replace an existing animation                |
| `DELETE` | `/anim/{id}` | Delete an animation                          |
| `GET`    | `/health`    | Health check (`{"status":"ok"}`)             |

---

## Credits

This project started as a fork of [`ascii-live`](https://github.com/hugomd/ascii-live) to support colored animations. The initial conversion logic relied on [this shell script](https://gist.github.com/teanup/17ee7938e1aa9e1d9d6b2f3fc611ad78).

It has since been rewritten from scratch to add a web interface, server-side conversion, and a more complete API.

Contributions, issues, and creative GIFs are welcome.
