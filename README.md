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

The database is a single file. Back it up with any file-level tool. For a consistent snapshot while running:

```sh
sqlite3 /data/aurelianprm.db ".backup /backup/aurelianprm-$(date +%Y%m%d).db"
```

Or use `VACUUM INTO` for a compacted copy:

```sh
sqlite3 /data/aurelianprm.db "VACUUM INTO '/backup/aurelianprm-$(date +%Y%m%d).db'"
```
