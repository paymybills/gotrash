# gotrash

`gotrash` is a high-performance, single-binary, self-destructing file sharing and pastebin server written in Go. 

Designed with a clean, distraction-free **Notion-like aesthetic**, it strips away complex layouts in favor of an elegant, minimalist light-themed workspace. Every text paste or file upload is configured with an expiration Time-To-Live (TTL) or a secure "Burn on Read" trigger, after which it is permanently purged from memory and disk.

---

## Features

- **Notion-Style UI:** Beautiful off-white canvas, thin gray structural borders, clean workspace panels, and status indicator tags.
- **Single Binary Portability:** Frontend HTML, CSS, and client-side JavaScript assets are packed directly into the compiled executable using Go's native `go:embed`.
- **Stateless/Stateful Hybrid:** Ephemeral metadata is held securely in-memory using concurrent mutex-protected maps, while file uploads stream directly to a designated persistent directory.
- **Background GC (Janitor):** A automated background sweeper ticks at regular intervals to purge expired shares and shred physical files off the disk.
- **CLI-Native Companion:** Full support for command-line uploads and piping via native `curl` inputs.
- **Docker-Ready:** Optimized multi-stage `Dockerfile` producing a tiny runtime image under 15MB.

---

## Running Locally

### 1. Build from Source
Ensure you have Go 1.22+ installed, then compile the zero-dependency executable:
```bash
go build -o gotrash
```

### 2. Start the Server
Run the binary:
```bash
./gotrash -port 8080 -clean 30s
```
- `-port`: The HTTP port to bind to (defaults to `8080`).
- `-clean`: How frequently the expired items janitor sweeps active storage (defaults to `30s`).
- `-dir`: Directory to store file uploads (defaults to `./data/uploads`).

Access the browser console at **`http://localhost:8080`**.

---

## CLI Companion API Reference

You don't need a browser to share secrets, code pastes, or binary files. Use your terminal!

### 1. Upload a Text Paste
Pipe standard terminal output directly to the server:
```bash
echo "Important error log traces" | curl -F "content=<-" http://localhost:8080/api/upload
```

### 2. Upload a File
Send a local image, zip, or configuration file:
```bash
curl -F "file=@photo.png" http://localhost:8080/api/upload
```

### 3. Customize Expiration & Self-Destruction
Add form arguments to customize the upload:
```bash
curl -F "content=my_secret_token" \
     -F "ttl=5m" \
     -F "burn=true" \
     http://localhost:8080/api/upload
```
- `ttl`: Allowed units: `5m`, `1h`, `4h`, `1d`, `7d`.
- `burn`: Set to `true` to immediately destroy the paste from the server after the first viewer opens it.

### Expected Response (JSON):
```json
{
  "id": "vomtmVMX",
  "is_file": false,
  "created_at": "2026-06-01T12:26:58Z",
  "expires_at": "2026-06-01T13:26:58Z",
  "burn_on_read": false,
  "delete_token": "fr1WnYGMt7WDMK5c"
}
```
View the raw text output by visiting `/raw/vomtmVMX`.

---

## Cloud Deployment (Railway.app)

Deploying `gotrash` on **[Railway.app](https://railway.app)** is automated and runs within the free resource limits:

1. Create a **New Project** on Railway.
2. Select **Deploy from GitHub repo** and choose `paymybills/gotrash`.
3. Go to the project **Settings** -> **Volumes** -> Click **Add Volume**:
   - **Mount Path:** `/app/data/uploads` (This ensures physical file persistence when deploying code changes).
4. Click **Generate Domain** under Networking.

Railway will build the multi-stage `Dockerfile`, automatically inject the dynamic `$PORT` environment variable, and deploy your instance instantly.
