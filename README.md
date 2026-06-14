# provider-webaudio

A [Crossplane](https://crossplane.io) provider that drives a browser-based 16-step musical sequencer using the [Web Audio API](https://developer.mozilla.org/en-US/docs/Web/API/Web_Audio_API). Declare your pattern in Kubernetes, hear it play in the browser.

Built as a teaching tool for platform engineering concepts — reconciler loops, drift detection, Compositions, XRDs, and Claims — with zero cloud credentials required.

---

## How it works

```
kubectl apply -f examples/
        │
        ▼
  Kubernetes API
  Sequencer / Track / Step CRDs
        │
        ▼
  Crossplane Reconciler
  ├── watches Sequencer, Track, Step resources
  ├── detects drift between spec and observed state
  ├── corrects divergence on every reconcile cycle
  └── pushes state over WebSocket to the browser
              │
              ▼
        Browser (localhost:9090)
        ├── 16-step grid renders spec state
        ├── Web Audio API plays the pattern
        └── step click → patch → reconciler corrects → UI reverts
```

The sequencer has 7 pitched rows (C4 through B4, standard tuning) and 16 steps per row. Each row is a `Track` resource; each cell is a `Step` resource. The reconciler enforces whatever `active: true/false` is declared in the spec — if the browser diverges, Kubernetes wins.

---

## Prerequisites

- [Go 1.22+](https://go.dev/dl/)
- [kubectl](https://kubernetes.io/docs/tasks/tools/) pointed at a cluster (local KinD works great)
- [controller-gen](https://github.com/kubernetes-sigs/controller-tools) for code generation
- Crossplane or UXP installed in the cluster ([install guide](https://docs.upbound.io/uxp/install/))
- A modern browser for the Web Audio API

---

## Quickstart

**1. Clone and install dependencies**

```bash
git clone https://github.com/example/provider-webaudio
cd provider-webaudio
go mod tidy
```

**2. Generate CRDs and apply them**

```bash
controller-gen object:headerFile="hack/boilerplate.go.txt" paths="./..."
controller-gen crd:allowDangerousTypes=true paths="./..." output:crd:artifacts:config=config/crd/bases
kubectl apply -f config/crd/bases/
```

**3. Apply the example resources**

```bash
kubectl apply -f examples/providerconfig.yaml
kubectl apply -f examples/sequencer.yaml
kubectl apply -f examples/tracks.yaml
kubectl apply -f examples/steps.yaml
```

**4. Run the provider**

```bash
go run ./cmd/provider --server-port 9090
```

**5. Open the sequencer**

Navigate to [http://localhost:9090](http://localhost:9090). You should see a 7×16 grid playing a C major arpeggio (C4–E4–G4–A4).

---

## Resource model

### Sequencer

The top-level resource. Owns all Tracks in the same namespace that reference it.

```yaml
apiVersion: webaudio.example.com/v1alpha1
kind: Sequencer
metadata:
  name: my-sequencer
  namespace: default
spec:
  bpm: 120      # 20–300
  steps: 16     # 4, 8, or 16
  running: true
```

### Track

One pitched row in the grid. Each Track references a Sequencer and carries the note's frequency.

```yaml
apiVersion: webaudio.example.com/v1alpha1
kind: Track
metadata:
  name: note-a
  namespace: default
spec:
  sequencerRef: my-sequencer
  instrument: synth
  waveform: sine        # sine | square | sawtooth | triangle
  frequency: 440.00     # Hz — standard tuning, A4 = 440 Hz
  volume: 0.6           # 0.0–1.0
```

Standard note frequencies (A4 = 440 Hz tuning):

| Note | Frequency |
|------|-----------|
| C4   | 261.63 Hz |
| D4   | 293.66 Hz |
| E4   | 329.63 Hz |
| F4   | 349.23 Hz |
| G4   | 392.00 Hz |
| A4   | 440.00 Hz |
| B4   | 493.88 Hz |

### Step

One cell in the grid. References a Track, carries the step index and whether it fires.

```yaml
apiVersion: webaudio.example.com/v1alpha1
kind: Step
metadata:
  name: note-a-step-3
  namespace: default
spec:
  trackRef: note-a
  index: 3          # 0–15
  active: true
  velocity: 0.8     # 0.0–1.0 — scales the note volume
```

---

## Changing the pattern

Toggle a single step on or off:

```bash
kubectl patch step note-a-step-3 \
  --type=merge \
  -p '{"spec":{"active":true}}'
```

Change the tempo:

```bash
kubectl patch sequencer my-sequencer \
  --type=merge \
  -p '{"spec":{"bpm":140}}'
```

Stop playback:

```bash
kubectl patch sequencer my-sequencer \
  --type=merge \
  -p '{"spec":{"running":false}}'
```

Mute a track:

```bash
kubectl patch track note-b \
  --type=merge \
  -p '{"spec":{"muted":true}}'
```

---

## Drift detection

Click any step in the browser. The step visually toggles — then snaps back after ~30 seconds.

What happened:

1. The browser sent a WebSocket patch to the provider.
2. The provider recorded the divergence in `status.observedActive` (without touching `spec.active`).
3. The reconciler noticed `spec.active ≠ status.observedActive`.
4. The reconciler corrected `status.observedActive` to match `spec.active` and pushed the corrected state to the browser.
5. The browser received the updated state and reverted the cell.

This is the core Crossplane pattern — the reconciler enforces declared state continuously. Watch it live:

```bash
kubectl get steps -w
```

---

## Repository structure

```
provider-webaudio/
├── cmd/provider/main.go              # Entrypoint — starts manager + HTTP server
├── internal/
│   ├── controller/
│   │   ├── sequencer.go              # Sequencer reconciler — builds and broadcasts state
│   │   ├── track.go                  # Track reconciler — creates audio nodes
│   │   └── step.go                   # Step reconciler — drift detection and correction
│   └── server/
│       ├── http.go                   # HTTP server and static file serving
│       └── websocket.go              # WebSocket hub — browser ↔ reconciler bridge
├── apis/webaudio/v1alpha1/
│   ├── sequencer_types.go
│   ├── track_types.go
│   ├── step_types.go
│   ├── providerconfig_types.go
│   └── zz_generated.deepcopy.go     # Generated — do not edit
├── config/crd/bases/                 # Generated CRD manifests
├── package/
│   ├── crossplane.yaml               # Provider package metadata
│   ├── xrd.yaml                      # XSequencer CompositeResourceDefinition
│   └── composition.yaml              # Composition (claim → Tracks + Steps)
├── web/
│   ├── index.html                    # Sequencer grid UI + WebSocket client
│   └── audio.js                      # Web Audio API synthesizer (NOTE_FREQ map, oscillators)
├── examples/
│   ├── providerconfig.yaml
│   ├── sequencer.yaml
│   ├── tracks.yaml                   # 7 pitched tracks, C4–B4 standard tuning
│   └── steps.yaml                    # 112 steps — C major arpeggio default pattern
├── hack/boilerplate.go.txt
├── go.mod
└── go.sum
```

---

## Crossplane concepts demonstrated

| Concept | Where to look |
|---------|---------------|
| Managed Resource | `Step` — the smallest unit of declared state |
| Reconciler loop | `internal/controller/step.go` — observe → compare → correct |
| Drift detection | `StepStatus.DriftDetected`, `status.observedActive` vs `spec.active` |
| Provider | `cmd/provider/main.go` — controller-runtime manager setup |
| ProviderConfig | `apis/webaudio/v1alpha1/providerconfig_types.go` |
| Composite Resource | `package/xrd.yaml` — the `XSequencer` API |
| Composition | `package/composition.yaml` — assembles Tracks + Steps from a claim |
| Claim | `XSequencer` → creates the full sequencer hierarchy |

---

## License

Apache 2.0
