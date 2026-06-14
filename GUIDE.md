# Platform Engineering with the BACK Stack
## Teaching Crossplane through a Web Audio Sequencer

A guide for platform engineers learning Crossplane for the first time.
Instead of provisioning cloud infrastructure, you'll build a provider that drives a
browser-based 4x4 step sequencer — same concepts, zero AWS bill.

---

## The core idea

Every Crossplane provider does one thing: it makes an external system match
what you declared in Kubernetes. The external system could be AWS, a database,
or — in this guide — a Web Audio engine running in a browser.

You declare a `Sequencer` resource. The reconciler watches it and drives the
browser to match. When state drifts (someone clicks a step in the UI), the
reconciler corrects it. The browser updates. You just watched Kubernetes win.

---

## Architecture

```
kubectl apply -f examples/
        │
        ▼
  Kubernetes API
  (Sequencer / Track / Step CRDs)
        │
        ▼
  Crossplane Reconciler  ←── watches for drift every 2s
  ├── sequencer.go            compares spec ↔ observed
  ├── track.go                corrects divergence
  ├── step.go                 reports status
  └── WebSocket server :9090
              │
              ▼  WebSocket
        Browser (index.html)
        ├── 4×4 grid renders spec state
        ├── Web Audio API plays the pattern
        └── step click → PATCH → reconciler detects drift → corrects → UI reverts
```

The drift loop is the lesson. Click a step in the browser. Watch it flash red
(drift detected). Watch it snap back (reconciler corrected). That's what every
Crossplane provider does, all day, for real infrastructure.

---

## Repository structure

```
provider-webaudio/
├── cmd/provider/main.go              # Entrypoint — starts manager + HTTP server
├── internal/
│   ├── controller/
│   │   ├── sequencer.go              # Sequencer reconciler
│   │   ├── track.go                  # Track reconciler
│   │   └── step.go                   # Step reconciler + drift detection
│   └── server/
│       ├── http.go                   # HTTP server setup
│       └── websocket.go              # WebSocket hub (browser ↔ reconciler)
├── apis/webaudio/v1alpha1/
│   ├── sequencer_types.go            # Sequencer CRD type definitions
│   ├── track_types.go                # Track CRD type definitions
│   ├── step_types.go                 # Step CRD type definitions
│   ├── providerconfig_types.go       # ProviderConfig type definitions
│   └── zz_generated.deepcopy.go     # Generated — do not edit
├── package/
│   ├── crossplane.yaml               # Provider package metadata
│   ├── xrd.yaml                      # XSequencer CompositeResourceDefinition
│   └── composition.yaml              # Composition (claim → Tracks + Steps)
├── web/
│   ├── index.html                    # 4×4 grid UI + WebSocket client
│   └── audio.js                      # Web Audio API synthesizer
└── examples/
    ├── providerconfig.yaml           # ProviderConfig (port, sample rate)
    ├── sequencer.yaml                # A Sequencer resource
    ├── tracks.yaml                   # Four Track resources
    ├── steps.yaml                    # 64 Step resources (4 tracks × 16 steps)
    ├── argocd-application.yaml       # Argo CD Applications for GitOps
    ├── kyverno-policies.yaml         # Four Kyverno policies
    └── backstage-catalog.yaml        # Backstage component + Software Template
```

---

## Chapter 1 — What is a Crossplane provider?

A Crossplane provider is a Kubernetes controller that manages resources in an
external system. It does three things, repeatedly, forever:

**Observe** — read the current state of the external system.
**Compare** — diff it against the desired state declared in a CRD spec.
**Correct** — update the external system to match the spec.

This cycle is called the reconciler loop. It runs on a timer (every 2 seconds
in this provider) and also triggers whenever a resource changes.

The key guarantee: **desired state always wins**. It doesn't matter what someone
does to the external system directly — the reconciler will correct it.

In this provider:
- The external system is the Web Audio engine in a browser
- The desired state is what you wrote in `Sequencer`, `Track`, and `Step` CRDs
- The reconciler loop is `controller/sequencer.go:Reconcile()`

### The three resource types

**Sequencer** — the top-level resource. Controls BPM, step count, and whether
the clock is running. One Sequencer = one pattern.

**Track** — one row in the grid. Controls which instrument plays (kick, snare,
hihat, synth) and its volume. A Track owns its Steps.

**Step** — one cell in the grid. Controls whether a step is active and at what
velocity. This is where drift detection lives.

### spec vs status

Every managed resource has two sections:

`spec` — what you want. You write this. The reconciler enforces it.
`status` — what actually exists. The reconciler writes this after observing.

If `spec.active: true` but `status.observedActive: false`, there is drift.
The reconciler's job is to close that gap.

---

## Chapter 2 — Provider SDK setup

This provider uses `controller-runtime` directly rather than Upjet (the
Terraform-based provider SDK). That's intentional for teaching — Upjet adds
a lot of machinery that obscures the reconciler loop.

For real cloud providers, use Upjet. For understanding how providers work, start here.

### Prerequisites

```bash
# Go 1.22+
go version

# controller-gen for code generation
go install sigs.k8s.io/controller-tools/cmd/controller-gen@latest

# crossplane CLI for packaging
curl -sL https://raw.githubusercontent.com/crossplane/crossplane/master/install.sh | sh
```

### Initialise the module

```bash
mkdir provider-webaudio && cd provider-webaudio
go mod init github.com/example/provider-webaudio
```

---

## Chapter 3 — Defining the APIs

The API types live in `apis/webaudio/v1alpha1/`. Each file defines a CRD.

### The Sequencer type

```go
// SequencerSpec is the desired state — what you declare.
type SequencerSpec struct {
    BPM     int  `json:"bpm"`
    Steps   int  `json:"steps"`
    Running bool `json:"running"`
}

// SequencerStatus is the observed state — what the reconciler reports.
type SequencerStatus struct {
    Ready           bool         `json:"ready,omitempty"`
    ObservedBPM     int          `json:"observedBPM,omitempty"`
    ObservedRunning bool         `json:"observedRunning,omitempty"`
    LastSyncTime    *metav1.Time `json:"lastSyncTime,omitempty"`
}
```

The split between spec and status is not cosmetic. `spec` is immutable from
the reconciler's perspective — it reads it but never writes it. `status` is
where the reconciler reports back what it observed and did.

### kubebuilder markers

The `// +kubebuilder:` comments above the types are markers that drive code
generation. Key ones:

```go
// +kubebuilder:validation:Minimum=20
// +kubebuilder:validation:Maximum=300
BPM int `json:"bpm"`
```

This generates a JSON Schema validation rule in the CRD. Kubernetes enforces
it at admission time — before the reconciler ever sees the resource.
Kyverno adds a second layer of validation on top.

### Fully qualified package URLs

Crossplane v2 enforces fully qualified image URLs in all package references.
You cannot use `upbound/provider-helm:v1.2.0` — you must use
`xpkg.crossplane.io/crossplane-contrib/provider-helm:v1.2.0`.

This applies to any `Provider`, `Configuration`, or `Function` resource.

---

## Chapter 4 — Code generation

After writing your types, run `controller-gen` to generate:
- The `zz_generated.deepcopy.go` file (required by controller-runtime)
- CRD YAML manifests (required for `kubectl apply`)
- RBAC manifests

```bash
# Generate deepcopy methods
controller-gen object:headerFile="hack/boilerplate.go.txt" paths="./..."

# Generate CRD manifests
controller-gen crd paths="./..." output:crd:artifacts:config=config/crd/bases

# Generate RBAC manifests from +kubebuilder:rbac markers
controller-gen rbac:roleName=provider-webaudio paths="./..."
```

**Never edit `zz_generated.deepcopy.go` by hand.** It is regenerated on every
`make generate` run. Changes will be overwritten.

---

## Chapter 5 — ProviderConfig

The `ProviderConfig` is how a provider receives credentials and configuration.
For cloud providers, this is where AWS keys or GCP service account JSON goes.

For this provider, there are no credentials — the Web Audio API runs in a browser
on localhost. The `ProviderConfig` carries non-secret configuration instead:
port number, sample rate, and drift correction interval.

```yaml
apiVersion: webaudio.example.com/v1alpha1
kind: ProviderConfig
metadata:
  name: default
spec:
  serverPort: 9090
  sampleRate: 44100
  driftCorrectionInterval: 2
```

A credential-less provider is unusual in production but common in tutorials.
The concept is identical — every Managed Resource references a ProviderConfig
to know how to talk to the external system.

---

## Chapter 6 — The reconciler

The reconciler is in `internal/controller/sequencer.go`. The `Reconcile()`
function is called by controller-runtime whenever:
- A `Sequencer` resource is created, updated, or deleted
- The `RequeueAfter` timer fires (every 2 seconds)
- A child resource (Track, Step) changes

The function follows the four-step pattern every Crossplane provider uses:

```go
func (r *SequencerReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {

    // 1. Observe — fetch the resource and the external system state
    var seq webaudiov1alpha1.Sequencer
    if err := r.Get(ctx, req.NamespacedName, &seq); err != nil {
        return ctrl.Result{}, client.IgnoreNotFound(err)
    }

    // 2. Compare — diff spec against observed
    observed := r.engine.sequencers[seq.Name]
    driftDetected := observed.BPM != seq.Spec.BPM || observed.Running != seq.Spec.Running

    // 3. Correct — update the external system
    state := r.buildStatePayload(ctx, &seq)
    r.Hub.Broadcast <- state

    // 4. Report — write observed state back to status
    seq.Status.ObservedBPM = seq.Spec.BPM
    r.Status().Update(ctx, &seq)

    // Requeue in 2s for continuous drift detection
    return ctrl.Result{RequeueAfter: 2 * time.Second}, nil
}
```

### The RequeueAfter guarantee

Returning `ctrl.Result{RequeueAfter: 2 * time.Second}` means: even if nothing
changes in Kubernetes, re-run this reconciler in 2 seconds. This is the
"always reconcile" guarantee — the external system cannot drift for more than
2 seconds without being caught and corrected.

For production providers this interval is typically longer (30s–5m) because
external API calls are expensive. For this provider it's 2 seconds to make
drift correction visible in the UI.

---

## Chapter 7 — Drift detection

Drift detection lives in `internal/controller/step.go`. The `StepReconciler`
is where the most important Crossplane concept is demonstrated.

### The drift loop

1. User clicks a step in the browser
2. Browser sends `{ type: "patch", payload: { stepName: "kick-step-3", active: true } }`
3. `watchPatches()` goroutine receives it and writes `status.observedActive = true`
   — but **does not touch `spec.active`**
4. The status update triggers a reconcile
5. `Reconcile()` compares `spec.active` (desired) with `status.observedActive` (observed)
6. They differ → drift detected → reconciler sets `status.observedActive = spec.active`
7. Status update triggers Sequencer reconciler
8. Sequencer reconciler broadcasts corrected state to browser
9. Browser receives state, re-renders grid cell to match spec

**The key insight:** the reconciler never changes `spec`. It only changes the
external system (and reports the result in `status`). `spec` is the contract
between the platform engineer and the system — it is never overridden.

### Watching drift in the terminal

```bash
# Watch Step resources in real time
kubectl get steps -w

# See drift status
kubectl get step kick-step-3 -o jsonpath='{.status}'
```

When you click a step in the browser, you will briefly see
`driftDetected: true` before the reconciler corrects it.

---

## Chapter 8 — The HTTP + WebSocket server

The server (`internal/server/`) is the bridge between the reconciler and the
browser. It has two responsibilities:

**Outbound (reconciler → browser):** The `Hub.Broadcast` channel receives
`StatePayload` structs from the reconciler after every sync. The `Run()` goroutine
fans these out to every connected WebSocket client.

**Inbound (browser → reconciler):** The `ServeWS` handler reads messages from
the browser. When it receives a `patch` message, it forwards it to the
`Hub.Patches` channel, which the `StepReconciler.watchPatches()` goroutine drains.

The server also serves the static web UI from the `web/` directory via
`http.FileServer`.

---

## Chapter 9 — The web app

The web app is two files: `web/index.html` and `web/audio.js`.

`index.html` handles:
- WebSocket connection and reconnection
- Rendering the 4×4 grid from the state payload
- Highlighting drift (red flash) and current playhead step
- Sending patch messages when a step is clicked

`audio.js` handles:
- Web Audio API context management
- Four instrument synthesizers (kick, snare, hihat, synth)
- The `playInstrument(instrument, volume, velocity)` dispatcher

### Opening the UI

Once the provider is running:

```bash
go run ./cmd/provider --server-port 9090
```

Open `http://localhost:9090` in a browser. The grid will be empty until
you apply the example manifests.

### Applying the pattern

```bash
kubectl apply -f examples/providerconfig.yaml
kubectl apply -f examples/sequencer.yaml
kubectl apply -f examples/tracks.yaml
kubectl apply -f examples/steps.yaml
```

The grid populates. Hit play (or patch the Sequencer):

```bash
kubectl patch sequencer my-sequencer \
  --type=merge \
  -p '{"spec":{"running":true}}'
```

Now click a step. Watch it flash red and snap back.

---

## Chapter 10 — Compositions and XRDs

A Composition is a template that assembles managed resources.
An XRD (CompositeResourceDefinition) defines the API for a composite resource.

Together they let platform engineers offer a higher-level API:
instead of writing Sequencer + Track + Step YAML, a developer writes a
Claim — a much simpler object.

### The XRD

`package/xrd.yaml` defines a `Sequencer` claim kind:

```yaml
spec:
  claimNames:
    kind: Sequencer      # What developers put in their namespace
  names:
    kind: XSequencer     # The composite the platform team works with
```

### The Composition

`package/composition.yaml` maps the claim's fields to managed resources.
A developer who declares:

```yaml
apiVersion: webaudio.example.com/v1alpha1
kind: Sequencer            # This is a Claim, not the MR
metadata:
  name: my-beat
  namespace: my-team
spec:
  bpm: 140
  steps: 16
  instruments: [kick, snare, hihat]
  pattern: fourOnTheFloor
```

Gets a fully assembled Sequencer + 3 Tracks + 48 Steps, all managed by Crossplane,
without writing any of that YAML themselves.

---

## Chapter 11 — Claims

A Claim is what a developer submits. It lives in their namespace.
The Composition creates the actual managed resources in the platform namespace.

This separation is the "self-service" part of platform engineering:
developers get what they need without needing cluster-admin access or
knowledge of the underlying provider.

```bash
# Developer submits a claim in their namespace
kubectl apply -f - <<EOF
apiVersion: webaudio.example.com/v1alpha1
kind: Sequencer
metadata:
  name: my-beat
  namespace: team-a
spec:
  bpm: 140
  steps: 16
  instruments: [kick, snare, hihat]
  pattern: fourOnTheFloor
EOF

# Platform watches the composite get assembled
kubectl get xsequencers
kubectl get sequencers,tracks,steps -n team-a
```

---

## Chapter 12 — Argo CD integration

Argo CD adds a third layer of enforcement: **Git is the source of truth**.

Apply the Argo CD Application:

```bash
# Edit YOUR_USERNAME first
kubectl apply -f examples/argocd-application.yaml
```

Now try editing a step directly with kubectl:

```bash
kubectl patch step kick-step-0 \
  --type=merge \
  -p '{"spec":{"active":false}}'
```

Within Argo CD's sync interval, it will revert the change back to what's in
Git. You now have two reconcilers:

1. **Argo CD** — enforces Git → Kubernetes
2. **Crossplane** — enforces Kubernetes CRD spec → Web Audio engine

A change has to survive both to actually affect the sequencer.

### The three layers of enforcement

| Layer | Enforces | Detects drift in |
|---|---|---|
| Kyverno | Policy → Kubernetes API | Admission (instant) |
| Argo CD | Git → Kubernetes | Every sync interval |
| Crossplane | Kubernetes spec → External system | Every 2 seconds |

---

## Chapter 13 — Kyverno policies

Apply the policies:

```bash
kubectl apply -f examples/kyverno-policies.yaml
```

### Testing the policies

```bash
# This should be rejected — no team label
kubectl apply -f - <<EOF
apiVersion: webaudio.example.com/v1alpha1
kind: Sequencer
metadata:
  name: unlabelled
  namespace: default
spec:
  bpm: 120
  steps: 16
  running: false
EOF
# Error: resource Sequencer was blocked due to the following policies
# require-team-label: ...

# This should be rejected — BPM too high
kubectl apply -f - <<EOF
apiVersion: webaudio.example.com/v1alpha1
kind: Sequencer
metadata:
  name: too-fast
  namespace: default
  labels:
    team: platform
spec:
  bpm: 500
  steps: 16
  running: false
EOF
# Error: BPM must be between 20 and 300. Got 500.
```

Kyverno fires at admission — before the resource is written to etcd,
before the reconciler ever sees it. It's the earliest possible enforcement point.

---

## Chapter 14 — Backstage integration

`examples/backstage-catalog.yaml` contains two things:

1. A `Component` that registers `provider-webaudio` in the Backstage catalog,
   with an Argo CD annotation that shows sync status inline.

2. A `Template` (Software Template) that lets developers provision a Sequencer
   from the Backstage UI without writing any YAML.

### Importing the catalog entry

In your Backstage instance:
1. Go to **Catalog → Register Existing Component**
2. Enter the URL to `examples/backstage-catalog.yaml` in your repo
3. The `provider-webaudio` component appears in the catalog

### Using the Software Template

1. Go to **Create → Web Audio Sequencer**
2. Fill in name, team, BPM, instruments
3. Backstage opens a PR to your GitOps repo
4. Merge the PR
5. Argo CD syncs the Sequencer claim
6. Crossplane reconciles it into Tracks and Steps
7. The component appears in the catalog with a link to the live UI

This is the full platform engineering story:
developer experience (Backstage) → GitOps (Argo CD) → policy (Kyverno) →
infrastructure (Crossplane) → external system (Web Audio).

---

## Chapter 15 — Running the full demo

### Start the provider

```bash
# Run against your local KinD cluster
go run ./cmd/provider \
  --server-port 9090 \
  --kubeconfig ~/.kube/config
```

### Apply everything

```bash
kubectl apply -f examples/providerconfig.yaml
kubectl apply -f examples/kyverno-policies.yaml
kubectl apply -f examples/sequencer.yaml
kubectl apply -f examples/tracks.yaml
kubectl apply -f examples/steps.yaml
```

### Open the UI

```
http://localhost:9090
```

### Start the sequencer

```bash
kubectl patch sequencer my-sequencer \
  --type=merge \
  -p '{"spec":{"running":true}}'
```

### Demonstrate drift

Click any active step in the browser. Observe:
1. Step flashes red — `status.driftDetected: true`
2. Within 2 seconds, step snaps back — reconciler corrected it
3. In the terminal: `kubectl get step <name> -w` shows the correction

### Change BPM

```bash
kubectl patch sequencer my-sequencer \
  --type=merge \
  -p '{"spec":{"bpm":160}}'
```

The browser immediately receives the new BPM and the scheduler adjusts.

### Trigger a Kyverno rejection

```bash
kubectl patch sequencer my-sequencer \
  --type=merge \
  -p '{"spec":{"bpm":999}}'
# Error from server: admission webhook denied the request
```

### Demonstrate Argo CD drift correction

```bash
# Direct edit (bypasses Git)
kubectl patch step kick-step-0 \
  --type=merge \
  -p '{"spec":{"active":false}}'

# Check Argo CD UI — the app will show OutOfSync
# After the sync interval, the step reverts to active: true
```

---

## Extending the provider

### Add a new instrument

1. Add `bass` to the `Instrument` enum in `track_types.go`
2. Implement `playBass()` in `web/audio.js`
3. Add a `bass` case to `playInstrument()`
4. Run `controller-gen crd paths="./..."` to regenerate CRDs
5. Apply the updated CRDs: `kubectl apply -f config/crd/bases/`

### Add a velocity-sensitive UI

The `Step.spec.velocity` field already exists. The `audio.js` synthesizers
accept a `velocity` parameter. Extend `index.html` to render step brightness
proportional to velocity, and add a right-click handler to adjust it.

### Add a cloud provider sidecar

Add `provider-family-aws` to your cluster and write a Composition that
provisions an S3 bucket per Sequencer (to store recorded patterns).
This bridges the tutorial to real infrastructure — same concepts, real cloud.
