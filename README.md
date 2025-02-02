# Kubernetes Event Operator

A Kubernetes operator that monitors deployments and statefulsets, taking proactive actions to maximize uptime and minimize disruptions.

## Overview

This operator monitors the status of specified deployments and statefulsets in your Kubernetes cluster, implementing automated responses to common issues and providing intelligent analysis of failures. It helps maintain high availability by:

- Detecting and analyzing pod failures
- Implementing automatic remediation steps
- Providing detailed diagnostics via Slack notifications
- Handling OOMKilled and CrashLoopBackOff scenarios
- Supporting vertical node scaling

## Features

### Current Features
- 🔍 Monitors health of specified deployments and statefulsets
- 🤖 Automatic pod failure detection and analysis
- 🧠 Intelligent failure analysis using AI/ML
- 📱 Slack notifications with detailed diagnostics
- 🔄 Automatic pod restart on container failures
- 💾 Support for OOMKilled and CrashLoopBackOff scenarios

### Upcoming Features
1. **Automatic Vertical Node Scaling**
   - Detect and respond to OOMKilled events
   - Cloud provider-agnostic interface
   - Rolling upgrade strategy for node scaling

2. **Enhanced Logging & Analysis**
   - Configurable log severity filtering (WARN, INFO, DEBUG)
   - AI-powered log analysis using k8sgpt
   - Customizable AI model integration
   - Detailed Slack notifications with:
     - Problem description
     - Remediation steps
     - Supporting evidence

## Prerequisites

- Kubernetes cluster 1.16+
- kubectl configured to communicate with your cluster
- Slack webhook URL (for notifications)
- Go 1.19+

## Installation

1. Install the Custom Resource Definition (CRD):
```bash
kubectl apply -f config/crd/bases/api.chil-pavn.online_notifiers.yaml
```

2. Deploy the operator:
```bash
kubectl apply -f config/samples/api_v1alpha1_notifier.yaml
```

## Configuration

The operator can be configured through the Notifier Custom Resource. Example configuration:

```yaml
apiVersion: api.chil-pavn.online/v1alpha1
kind: Notifier
metadata:
  name: notifier-sample
spec:
  # Configuration details here
```

## Development

### Building from Source

1. Clone the repository:
```bash
git clone https://github.com/chil-pavn/kube-event-operator.git
cd kube-event-operator
```

2. Install dependencies:
```bash
go mod download
```

3. Build the operator:
```bash
make build
```

### Running Tests

```bash
make test
```

### Building Docker Image

```bash
make docker-build docker-push IMG=<your-registry>/kube-event-operator:tag
```

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request. For major changes, please open an issue first to discuss what you would like to change.

1. Fork the Project
2. Create your Feature Branch (`git checkout -b feature/AmazingFeature`)
3. Commit your Changes (`git commit -m 'Add some AmazingFeature'`)
4. Push to the Branch (`git push origin feature/AmazingFeature`)
5. Open a Pull Request
