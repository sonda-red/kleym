---
title: Usage
weight: 10
description: "Usage reference for kleym status and kleym inspect binding, including flags, output modes, kubeconfig handling, and strict inspection."
---

Build the CLI from a checkout:

```sh
make build-cli
```

The local binary is written to `bin/kleym`.

Or download the latest released Linux or macOS CLI without building:

```sh
mkdir -p bin
version="$(curl -fsSLI -o /dev/null -w '%{url_effective}' https://github.com/sonda-red/kleym/releases/latest | sed 's#.*/##')"
os="$(uname -s | tr '[:upper:]' '[:lower:]')"
case "$(uname -m)" in
  x86_64) arch="amd64" ;;
  arm64|aarch64) arch="arm64" ;;
  *) echo "unsupported architecture: $(uname -m)" >&2; exit 1 ;;
esac
curl -fsSL "https://github.com/sonda-red/kleym/releases/download/${version}/kleym_${version}_${os}_${arch}.tar.gz" | tar -xz -C bin kleym
chmod +x bin/kleym
```

Release archives are published for `linux_amd64`, `linux_arm64`,
`darwin_amd64`, `darwin_arm64`, and `windows_amd64`.

Print the linked version without contacting Kubernetes:

```sh
bin/kleym --version
```

Show a cluster overview:

```sh
bin/kleym status
```

Inspect one binding:

```sh
bin/kleym inspect binding <name> -n <namespace>
```

Inspection normally reads operator config from `InferenceIdentityBinding.status.trustDomain`
and `status.clusterSPIFFEIDClassName`. Pass flags only when you need to override
that discovered config or inspect an older binding whose status does not record it:

```sh
bin/kleym inspect binding <name> -n <namespace> \
  --trust-domain=example.org \
  --clusterspiffeid-class-name=kleym
```

Use JSON for automation:

```sh
bin/kleym status -o json
bin/kleym inspect binding <name> -n <namespace> -o json
```

## Flags

| Flag | Meaning |
| --- | --- |
| `-n`, `--namespace` | Binding namespace for `inspect binding`. Defaults to `default`. |
| `-o`, `--output` | Output format: `text` or `json`. Defaults to `text`. |
| `--strict` | Treat warning-severity findings as an inspection issue exit for `inspect binding`. |
| `--context` | Kubeconfig context name. |
| `--kubeconfig` | Kubeconfig file path. |
| `--timeout` | Command timeout. Must be greater than zero. |
| `--trust-domain` | Override trust domain used by `inspect binding` to render SPIFFE IDs. If operator config is unavailable and this flag is omitted, inspection falls back to `kleym.sonda.red`. |
| `--clusterspiffeid-class-name` | Override expected `ClusterSPIFFEID.spec.className` for `inspect binding`. If operator config is unavailable and this flag is omitted, inspection falls back to classless output. |

## Access

The CLI needs Kubernetes API access to read the requested resources. A
permission, connection, authentication, or discovery failure that prevents
status evaluation or binding inspection is fatal and may not emit a complete
report.

After the binding is readable, limited access to GAIE resources or pods is
reported through findings when inspection can continue.

Full inspection may need read access to:

- Kleym operator Deployments for `kleym status`
- `InferenceIdentityBinding` resources for `kleym status`
- `InferenceIdentityBinding` in the binding namespace
- supported GAIE `InferencePool` and `InferenceObjective` resources
- pods in the binding namespace when matched pod reporting is enabled

## Boundary

`kleym status` and `kleym inspect binding` are read-only. They do not create,
update, delete, reconcile, or patch Kubernetes resources.
