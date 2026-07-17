# Ruleset Guide

This file summarizes the rule families used by `config/rules.yml`.

## Generic Rules

- Email addresses.
- Phone numbers.
- Username, password, token, API key, and secret fields in JSON, YAML, or logs.
- Authorization and Cookie headers.
- Credentials embedded in connection strings.

## Service-Specific Rules

- Java: system properties and SSL/password flags.
- Tomcat: manager/deployer credentials and runtime options.
- WildFly: admin and management credentials.
- Apache: auth headers and auth-related directives.
- MySQL: environment variables, CLI flags, JDBC URLs.
- OracleDB: SQL*Plus and wallet/password fields.
- LDAP: bind passwords, URIs, and distinguished names.
- SSH: `sshpass`, password fields, and key comments.
- PowerShell: password parameters and plaintext secure string input.

## Usage Guidance

- Keep patterns deterministic and focused on secret-bearing fields.
- Prefer explicit capture groups so replacements preserve surrounding syntax.
- Add regression tests whenever a rule is added or changed.
- Validate changes with `go test ./...` and the Docker-based test stack.
