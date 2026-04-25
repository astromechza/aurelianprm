# aurelianprm

A personal relationship manager. Track people, organisations, addresses, and the relationships between them. Stores everything in a single SQLite file.

## Installation

### Binary

Download a pre-built binary from [Releases](https://github.com/astromechza/aurelianprm/releases/latest):

```sh
# macOS arm64
curl -L https://github.com/astromechza/aurelianprm/releases/latest/download/aurelianprm_darwin_arm64.tar.gz | tar xz
./aurelianprm --db ./aurelianprm.db
```

Available targets: `linux_amd64`, `linux_arm64`, `darwin_amd64`, `darwin_arm64`, `windows_amd64`, `windows_arm64`.

### Container image

```
ghcr.io/astromechza/aurelianprm:latest
```

Multi-arch manifest (linux/amd64, linux/arm64). Runs as non-root (uid 65532, distroless).

### Build from source

Requires Go 1.26+.

```sh
git clone https://github.com/astromechza/aurelianprm.git
cd aurelianprm
go build -o aurelianprm .
```

## Configuration

| Flag | Default | Description |
|------|---------|-------------|
| `--db` | required | Path to SQLite database file |
| `--addr` | `:8080` | Listen address |

The database file is created automatically on first run. Schema migrations run at startup.

## Data volume

The container writes a single SQLite file at `/data/aurelianprm.db`. The `/data` directory **must be writable by uid 65532** (the distroless nonroot user).

```sh
# On the host, before mounting:
mkdir -p ./data
chown -R 65532:65532 ./data
```

---

## Docker

### Quick start

```sh
docker run -d \
  --name aurelianprm \
  -p 8080:8080 \
  -v $(pwd)/data:/data \
  --user 65532:65532 \
  ghcr.io/astromechza/aurelianprm:latest
```

### Docker Compose

```yaml
services:
  aurelianprm:
    image: ghcr.io/astromechza/aurelianprm:latest
    restart: unless-stopped
    ports:
      - "8080:8080"
    volumes:
      - aurelianprm_data:/data
    user: "65532:65532"

volumes:
  aurelianprm_data:
    driver: local
```

Named volume ownership is managed by Docker. If you use a bind mount instead, `chown 65532:65532` the host directory first.

---

## Kubernetes

Singleton StatefulSet — one replica, one PVC. Do not run multiple replicas; SQLite is not safe for concurrent writers.

```yaml
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: aurelianprm
spec:
  serviceName: aurelianprm
  replicas: 1
  selector:
    matchLabels:
      app: aurelianprm
  template:
    metadata:
      labels:
        app: aurelianprm
    spec:
      securityContext:
        runAsUser: 65532
        runAsGroup: 65532
        fsGroup: 65532          # ensures PVC mounted with correct group ownership
      containers:
        - name: aurelianprm
          image: ghcr.io/astromechza/aurelianprm:latest
          ports:
            - containerPort: 8080
          volumeMounts:
            - name: data
              mountPath: /data
          securityContext:
            readOnlyRootFilesystem: true   # root FS is read-only; only /data is writable
          resources:
            requests:
              cpu: 50m
              memory: 64Mi
            limits:
              memory: 256Mi
          readinessProbe:
            httpGet:
              path: /
              port: 8080
            initialDelaySeconds: 5
            periodSeconds: 10
          livenessProbe:
            httpGet:
              path: /
              port: 8080
            initialDelaySeconds: 10
            periodSeconds: 30
  volumeClaimTemplates:
    - metadata:
        name: data
      spec:
        accessModes: [ReadWriteOnce]
        resources:
          requests:
            storage: 1Gi
---
apiVersion: v1
kind: Service
metadata:
  name: aurelianprm
spec:
  selector:
    app: aurelianprm
  ports:
    - port: 80
      targetPort: 8080
  type: ClusterIP
```

`fsGroup: 65532` causes Kubernetes to `chown` the mounted PVC to the group on pod start, so no manual ownership fix is needed.

### Ingress (optional)

```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: aurelianprm
spec:
  rules:
    - host: prm.example.com
      http:
        paths:
          - path: /
            pathType: Prefix
            backend:
              service:
                name: aurelianprm
                port:
                  number: 80
```

Add TLS via cert-manager or your ingress controller of choice. The app itself has no authentication — protect it at the ingress layer (basic auth, OAuth proxy, VPN, etc.).

---

## Backups

The database is a single SQLite file. The container image does not include shell utilities or `sqlite3`, so **the recommended approach is the built-in HTTP backup endpoint**.

### HTTP backup endpoint

```
GET /api/backup
```

Returns a complete, transactionally-consistent snapshot of the database as a downloadable `.db` file. The snapshot is created using `VACUUM INTO`, which is safe while the server is running and works with WAL mode. The temporary file is written alongside the database in `/data` (not in `/tmp`), so the container root filesystem can remain read-only.

The response includes a `Digest` header (RFC 9530) with the SHA-256 checksum of the body, so you can verify that the transfer was not corrupted:

```sh
# Download and verify integrity
FILE=backup-$(date +%Y%m%d).db
curl -D headers.txt -o "$FILE" http://localhost:8080/api/backup

# Extract the expected hash from the Digest header (sha-256=:<base64>:)
EXPECTED=$(grep -i '^Digest:' headers.txt | sed 's/.*sha-256=://;s/:.*//' | tr -d '\r' | base64 -d | xxd -p -c 256)
ACTUAL=$(sha256sum "$FILE" | awk '{print $1}')
[ "$EXPECTED" = "$ACTUAL" ] && echo "OK: integrity verified" || echo "FAIL: checksum mismatch"
```

### Docker: scheduled backup with cron

Run a cron job on the host (or a sidecar container) that downloads the backup and pushes it with rclone:

```sh
# crontab entry: daily at 02:00
0 2 * * * curl -sf http://localhost:8080/api/backup -o /tmp/aurelianprm-backup.db && \
           rclone copyto /tmp/aurelianprm-backup.db remote:backups/aurelianprm/$(date +\%Y\%m\%d).db && \
           rm /tmp/aurelianprm-backup.db
```

### Kubernetes: CronJob backup

A CronJob that fetches the backup from the service and uploads it with rclone:

```yaml
apiVersion: batch/v1
kind: CronJob
metadata:
  name: aurelianprm-backup
spec:
  schedule: "0 2 * * *"    # daily at 02:00 UTC
  concurrencyPolicy: Forbid
  jobTemplate:
    spec:
      template:
        spec:
          restartPolicy: OnFailure
          containers:
            - name: backup
              image: rclone/rclone:latest
              command:
                - /bin/sh
                - -c
                - |
                  set -e
                  DATE=$(date +%Y%m%d)
                  wget -q -O /tmp/backup.db http://aurelianprm/api/backup
                  rclone copyto /tmp/backup.db "${RCLONE_DEST}/aurelianprm-${DATE}.db"
              env:
                - name: RCLONE_DEST
                  value: "remote:backups"   # adjust to your rclone remote and path
              volumeMounts:
                - name: rclone-config
                  mountPath: /config/rclone
                  readOnly: true
          volumes:
            - name: rclone-config
              secret:
                secretName: rclone-config   # kubectl create secret generic rclone-config --from-file=rclone.conf
```

The CronJob accesses the database through the Kubernetes Service rather than mounting the PVC, so there is no ReadWriteOnce conflict.

### Direct file backup (requires volume access)

If you have shell access to a node or container that can mount the same volume, you can use `sqlite3` directly for a consistent snapshot:

```sh
sqlite3 /data/aurelianprm.db ".backup /backup/aurelianprm-$(date +%Y%m%d).db"
```

Or use `VACUUM INTO` for a compacted copy:

```sh
sqlite3 /data/aurelianprm.db "VACUUM INTO '/backup/aurelianprm-$(date +%Y%m%d).db'"
```

