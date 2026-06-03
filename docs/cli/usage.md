---
title: Usage
weight: 10
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

Inspect one binding:

```sh
bin/kleym inspect binding <name> -n <namespace>
```

If the operator was installed with non-default identity settings, pass the same
values to inspection:

```sh
bin/kleym inspect binding <name> -n <namespace> \
  --trust-domain=example.org \
  --clusterspiffeid-class-name=kleym
```

Use JSON for automation:

```sh
bin/kleym inspect binding <name> -n <namespace> -o json
```

## Flags

| Flag | Meaning |
| --- | --- |
| `-n`, `--namespace` | Binding namespace. Defaults to `default`. |
| `-o`, `--output` | Output format: `text` or `json`. Defaults to `text`. |
| `--strict` | Treat warning-severity findings as an inspection issue exit. |
| `--context` | Kubeconfig context name. |
| `--kubeconfig` | Kubeconfig file path. |
| `--timeout` | Inspection timeout. Must be greater than zero. |
| `--trust-domain` | Trust domain used to recompute desired SPIFFE IDs. Defaults to `kleym.sonda.red`. |
| `--clusterspiffeid-class-name` | Optional expected `ClusterSPIFFEID.spec.className`. Defaults to classless output. |

## Access

The CLI needs Kubernetes API access to read the requested
`InferenceIdentityBinding`. A permission, connection, authentication, or
discovery failure before the binding can be read is fatal and may not emit a
complete report.

After the binding is readable, limited access to peer bindings, GAIE resources,
managed `ClusterSPIFFEID` resources, or pods is reported through findings and
capability states when inspection can continue.

Full inspection may need read access to:

- `InferenceIdentityBinding` in the binding namespace
- supported GAIE `InferencePool` and `InferenceObjective` resources
- Kleym-managed `ClusterSPIFFEID` resources
- pods in the binding namespace when eligible workload reporting is desired

## Boundary

`kleym inspect binding` is read-only. It does not create, update, delete,
reconcile, or patch Kubernetes resources.
