# Porkbun DDNS Client

A lightweight dynamic DNS client for Porkbun, supporting both IPv4 (A records) and IPv6 (AAAA records) with multi-subdomain configuration. Took a lot of inspiration from https://github.com/luxeon/porkbun-ddns/, but I wanted AAAA and multiple subdomains (and to be able to update the base domain).

## Features

- Automatic DNS record updates for Porkbun domains
- Support for both A (IPv4) and AAAA (IPv6) records
- Multiple subdomain management
- Interactive configuration setup
- Minimal dependencies

## Installation

This tool was created to provide a simple, maintainable alternative to existing solutions that either lack package manager availability or have structural issues (I can't get the Config import to work on that python one as of this writing).

## Building from Source

To build the project:

```bash
go build -o porkbun-ddns ./cmd/porkbun-ddns
```

This will create a `porkbun-ddns` executable in the current directory.

## Usage

To create a configuration file interactively:

```bash
porkbun-ddns --create
```

Once configured, run the client to update your DNS records:

```bash
porkbun-ddns
```

The client will read your configuration and update the specified DNS records with your current IP addresses.

## Requirements

- Porkbun API credentials (API key and secret key)
- A registered domain on Porkbun with API access enabled
