.PHONY: generate build run apply-examples clean

# Generate CRDs, deepcopy, and RBAC from type annotations
generate:
	controller-gen object:headerFile="hack/boilerplate.go.txt" paths="./..."
	controller-gen crd paths="./..." output:crd:artifacts:config=config/crd/bases
	controller-gen rbac:roleName=provider-webaudio paths="./..."

# Build the provider binary
build:
	go build -o bin/provider ./cmd/provider

# Run against the current kubeconfig context (for development)
run:
	go run ./cmd/provider --server-port 9090

# Apply all example manifests to the cluster
apply-examples:
	kubectl apply -f examples/providerconfig.yaml
	kubectl apply -f examples/kyverno-policies.yaml
	kubectl apply -f examples/sequencer.yaml
	kubectl apply -f examples/tracks.yaml
	kubectl apply -f examples/steps.yaml

# Start the sequencer
start:
	kubectl patch sequencer my-sequencer --type=merge -p '{"spec":{"running":true}}'

# Stop the sequencer
stop:
	kubectl patch sequencer my-sequencer --type=merge -p '{"spec":{"running":false}}'

# Watch resources live
watch:
	watch -n1 "kubectl get sequencers,tracks,steps"

# Delete all example resources
clean:
	kubectl delete -f examples/steps.yaml --ignore-not-found
	kubectl delete -f examples/tracks.yaml --ignore-not-found
	kubectl delete -f examples/sequencer.yaml --ignore-not-found
	kubectl delete -f examples/providerconfig.yaml --ignore-not-found
