# Despliegue de JAPpi en K3s

> **Estado:** estos manifiestos corresponden a los binarios actuales. El Sprint 3 añadirá el backend Go único, la app Next.js y el entorno reproducible en Hetzner; Diego documentará sus pasos, backups y restauración. Ver [roadmap](../../docs/roadmap.md) y [ADR-0013](../../docs/adr/0013-monolito-modular-hasta-validar-el-producto.md).

Estado: **borrador**, hasta tener el VPS (ver [roadmap](../../docs/roadmap.md)). El orden es importante.

## 1. Nodo servidor

```bash
curl -sfL https://get.k3s.io | INSTALL_K3S_EXEC="server --secrets-encryption" sh -
```

`--secrets-encryption` cifra los Secrets en reposo (credenciales de los add-ons).

## 2. gVisor en cada nodo que ejecute código de clientes

Sigue la [guía oficial de instalación de runsc](https://gvisor.dev/docs/user_guide/install/), y después registra el runtime en el containerd de K3s:

```toml
# /var/lib/rancher/k3s/agent/etc/containerd/config.toml.tmpl
[plugins."io.containerd.grpc.v1.cri".containerd.runtimes.runsc]
  runtime_type = "io.containerd.runsc.v1"
```

Reinicia K3s y aplica `system.yaml`, que crea la `RuntimeClass` gvisor. Mientras un nodo no tenga gVisor, arranca el deployer con `RUNTIME_CLASS` vacío.

## 3. TLS wildcard (ADR-0009)

1. Instala cert-manager.
2. Crea un `ClusterIssuer` de Let's Encrypt con el **desafío DNS-01** de nuestro proveedor DNS (se decide al comprar el dominio).
3. Emite `*.jappi.app` en el namespace `kube-system`.
4. Configúralo como certificado por defecto de Traefik (`TLSStore` llamado `default`). Así las Ingress de los clientes no necesitan su propio certificado.

## 4. Plataforma

```bash
kubectl apply -f deploy/k8s/system.yaml
# NATS con JetStream (3 réplicas): chart oficial nats/nats con config.jetstream.enabled=true
kubectl apply -f deploy/k8s/deployer.yaml
```

## Qué puede hacer cada servicio

| Servicio | Permisos en el clúster |
|----------|------------------------|
| deployer | Namespaces, cuotas, LimitRanges, Services, Deployments, NetworkPolicies, Ingress (crear y aplicar); pods (solo lectura). **Sin Secrets.** |
| addons (pendiente) | Secrets y recursos de CloudNativePG, solo en los namespaces `prj-*` |
| resto | Ninguno: no hablan con Kubernetes |

**Riesgo asumido:** los permisos de un ClusterRole no se pueden limitar a los namespaces `prj-*`, así que el deployer podría escribir en cualquier namespace. Lo compensa que su única entrada son eventos de NATS, que solo publica el control-plane. Cuando crezcamos, un admission policy (`ValidatingAdmissionPolicy`) puede restringirlo a `prj-*`.
