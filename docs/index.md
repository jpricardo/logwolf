---
layout: home

hero:
  name: Logwolf
  text: Self-hosted event logging and observability.
  tagline: Your logs stay on your server. One docker-compose up and you're logging.
  actions:
    - theme: brand
      text: Get started
      link: /getting-started
    - theme: alt
      text: JS SDK reference
      link: /sdk/js

features:
  - title: Your data, your server
    details: No vendor lock-in, no per-seat pricing, no data leaving your infrastructure. Run Logwolf on any VPS with Docker.
  - title: One command to start
    details: The full stack — broker, listener, logger, dashboard, and MongoDB — orchestrated with a single docker-compose up.
  - title: Built for developers
    details: A clean JS SDK with sampling controls, automatic duration tracking, and Zod-validated config. Instrument anything in minutes.
  - title: Secure by default
    details: GitHub OAuth for the dashboard, API key auth for the SDK, TLS via Caddy, and an internal Docker network that keeps MongoDB and RabbitMQ off the internet.
---
