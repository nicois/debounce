# debounce

Run a command, but not too often

## Installation

## Usage

```bash
debounce <cooldown period> command <args>...
```

## Examples

Download an image, if not run in the past week

```bash
debounce 168h podman pull registry.fedoraproject.org/fedora:latest
```
