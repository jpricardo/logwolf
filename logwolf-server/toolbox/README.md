# Logwolf Toolbox

The **Toolbox** is a shared Go library used across the Logwolf microservices ecosystem. It centralizes common logic, data structures, and utility functions to ensure consistency and reduce code duplication between the services.

## Overview

This module is imported by the **Broker**, **Listener**, and **Logger** services. It provides standard implementations for:

- **Data Models**: Shared structs for log entries and MongoDB interactions.
- **Event Handling**: Common logic for connecting to RabbitMQ, publishing events, and consuming messages.
- **JSON Utilities**: Standardized helpers for reading requests and writing JSON responses.

## Usage

This directory functions as a local Go module defined in the project workspace (`go.work`). It is a library and is not intended to be executed as a standalone service.
